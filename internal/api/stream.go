package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/events"
)

/*
  SSE поток.

  Зачем: данные стейт-полла (3–5 с) для админа ощущаются «лагающими» — особенно
  тикеры скорости и журнал событий. Полл такой же по нагрузке как сейчас, но
  push даёт UI ощущение «вживую».

  Два типа сообщений:
    • event="tick"  — каждую секунду: суммарная скорость + per-client мгновенные
                       Rx/Tx за последнюю секунду. Считается diff'ом kernel-
                       счётчиков из awg show dump. Counter reset (после
                       awg-quick down/up) сглаживаем выкидыванием отрицательной
                       дельты.
    • event="audit" — мгновенно при events.Log.Append (через хук Subscribe).

  Heartbeat: ":\n\n" (SSE-комментарий) раз в 15 секунд — держит TCP живым
  через любые прокси и помогает фронту понять, что соединение цело.
*/

type Broker struct {
	mu    sync.RWMutex
	subs  map[chan []byte]struct{}
	nextN atomic.Uint64

	mgr   *awg.Manager
	cfg   brokerCfg
	tickC chan struct{}
}

type brokerCfg struct {
	Bin  string
	Tick time.Duration
}

func NewBroker(mgr *awg.Manager, bin string) *Broker {
	return &Broker{
		subs:  map[chan []byte]struct{}{},
		mgr:   mgr,
		cfg:   brokerCfg{Bin: bin, Tick: time.Second},
		tickC: make(chan struct{}, 1),
	}
}

// AttachEventLog подключается к events.Log: каждый Append отправляется в
// broadcast в виде SSE-сообщения с event="audit". Старый sink (для журнала
// в БД) остаётся, добавляется второй слушатель.
func (b *Broker) AttachEventLog(log *events.Log) {
	log.Subscribe(func(ev events.Event) {
		body, _ := json.Marshal(ev)
		b.send("audit", body)
	})
}

// Run запускает фоновый цикл, который раз в Tick читает счётчики и шлёт
// "tick" в broadcast. Завершается по ctx.Done(). Стоит запустить один раз
// из main.
func (b *Broker) Run(ctx context.Context) {
	t := time.NewTicker(b.cfg.Tick)
	defer t.Stop()

	type prev struct {
		rx, tx uint64
	}
	last := map[string]prev{}
	var lastT time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			// Если никто не подписан — не дёргаем kernel зря.
			b.mu.RLock()
			n := len(b.subs)
			b.mu.RUnlock()
			if n == 0 {
				last = map[string]prev{}
				lastT = time.Time{}
				continue
			}

			status := map[string]awg.PeerStatus{}
			for _, iface := range b.mgr.IfaceNames() {
				st, err := awg.ShowDump(b.cfg.Bin, iface)
				if err != nil {
					continue
				}
				for k, v := range st {
					status[k] = v
				}
			}
			dt := now.Sub(lastT).Seconds()
			if lastT.IsZero() || dt <= 0 || dt > 5 {
				dt = 0
			}
			lastT = now

			snap := b.mgr.Snapshot() // pubkey -> Client (id, name)
			type live struct {
				ID     string  `json:"id"`
				RxRate float64 `json:"rxRate"`
				TxRate float64 `json:"txRate"`
				Online bool    `json:"online"`
			}
			perClient := make([]live, 0, len(status))
			var sumRx, sumTx float64
			online := 0

			for pub, st := range status {
				c, ok := snap[pub]
				if !ok {
					continue
				}
				var rxRate, txRate float64
				if p, ok2 := last[pub]; ok2 && dt > 0 {
					if st.RxBytes >= p.rx {
						rxRate = float64(st.RxBytes-p.rx) / dt
					}
					if st.TxBytes >= p.tx {
						txRate = float64(st.TxBytes-p.tx) / dt
					}
				}
				last[pub] = prev{rx: st.RxBytes, tx: st.TxBytes}

				isOnline := st.LatestHandshake != nil && now.Sub(*st.LatestHandshake) < 3*time.Minute
				if isOnline {
					online++
				}
				sumRx += rxRate
				sumTx += txRate
				perClient = append(perClient, live{
					ID: c.ID, RxRate: rxRate, TxRate: txRate, Online: isOnline,
				})
			}

			payload := map[string]any{
				"ts":        now.Unix(),
				"rxRate":    sumRx,
				"txRate":    sumTx,
				"online":    online,
				"clients":   perClient,
			}
			body, _ := json.Marshal(payload)
			b.send("tick", body)
		}
	}
}

func (b *Broker) send(event string, data []byte) {
	frame := []byte("event: " + event + "\ndata: " + string(data) + "\n\n")
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- frame:
		default:
			// Подписчик не успевает читать — пропускаем кадр, чтобы не блочить
			// весь broadcast. Следующий tick догонит состояние.
		}
	}
}

func (b *Broker) subscribe() chan []byte {
	ch := make(chan []byte, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// stream — HTTP handler для SSE. Регистрируется под защитой auth middleware.
func (b *Broker) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // отключаем nginx-буфер

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	id := b.nextN.Add(1)
	// Hello-сообщение: фронт сразу понимает, что подписался.
	fmt.Fprintf(w, "event: hello\ndata: {\"id\":%s}\n\n", strconv.FormatUint(id, 10))
	flusher.Flush()

	hb := time.NewTicker(15 * time.Second)
	defer hb.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-ch:
			if _, err := w.Write(frame); err != nil {
				return
			}
			flusher.Flush()
		case <-hb.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
