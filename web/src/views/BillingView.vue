<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CalendarDays, Check, Clock3, ReceiptText, Users } from 'lucide-vue-next'
import { api } from '@/lib/api'
import { useSubscribersStore } from '@/stores/subscribers'
import { useToastStore } from '@/stores/toasts'
import { useTitle } from '@/composables/useTitle'
import type { BillingCycle, BillingInvoice } from '@/types'
import TopBar from '@/components/organisms/TopBar.vue'
import Button from '@/components/atoms/Button.vue'
import Badge from '@/components/atoms/Badge.vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Skeleton from '@/components/atoms/Skeleton.vue'

useTitle(() => 'Расходы · Amnezia Panel')

const subs = useSubscribersStore()
const toasts = useToastStore()
const cycles = ref<BillingCycle[]>([])
const selected = ref<BillingCycle | null>(null)
const received = ref(0)
const pending = ref(0)
const loading = ref(true)
const detailLoading = ref(false)
const createOpen = ref(false)
const creating = ref(false)
const publishing = ref<number | null>(null)
const paying = ref<number | null>(null)
const canceling = ref<number | null>(null)
const closing = ref<number | null>(null)
const deleting = ref<number | null>(null)

const pad = (n: number) => String(n).padStart(2, '0')
function dateInput(d: Date) { return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` }
function addDays(d: Date, days: number) { const out = new Date(d); out.setDate(out.getDate() + days); return out }

const now = new Date()
const monthStart = new Date(now.getFullYear(), now.getMonth(), 1)
const monthEnd = new Date(now.getFullYear(), now.getMonth() + 1, 0)
const draft = ref({
	 title: now.toLocaleDateString('ru-RU', { month: 'long', year: 'numeric' }),
	 total: '',
	 periodStart: dateInput(monthStart),
	 periodEnd: dateInput(monthEnd),
	 paymentDueAt: dateInput(addDays(monthEnd, 5)),
	 graceEndsAt: dateInput(addDays(monthEnd, 8)),
})

const payerCount = computed(() => subs.items.filter(s => s.billingRole === 'payer').length)
const previewShare = computed(() => {
	 const amount = Math.round(Number(String(draft.value.total).replace(',', '.')) * 100)
	 return payerCount.value && amount > 0 ? Math.floor(amount / payerCount.value) : 0
})

function money(kopecks: number) {
	 return new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB' }).format(kopecks / 100)
}
function day(ts: number) {
	 return new Date(ts * 1000).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' })
}
function unix(date: string) { return Math.floor(new Date(`${date}T12:00:00`).getTime() / 1000) }
function statusTone(status: string) {
	 if (status === 'paid') return 'success'
	 if (status === 'published' || status === 'pending') return 'warning'
	 return 'neutral'
}

async function load() {
	 try {
		 const [list, totals] = await Promise.all([api.billingCycles(), api.billingSummary(), subs.fetch(true)])
		 cycles.value = list || []
		 received.value = totals.totalReceived
		 pending.value = totals.totalPending
	 } catch (e: any) {
		 toasts.error(e?.message || 'Не удалось загрузить расчёты')
	 } finally { loading.value = false }
}

async function openCycle(cycle: BillingCycle) {
	 detailLoading.value = true
	 try { selected.value = await api.billingCycle(cycle.id) }
	 catch (e: any) { toasts.error(e?.message || 'Не удалось загрузить счета') }
	 finally { detailLoading.value = false }
}

async function createCycle() {
	 const totalAmount = Math.round(Number(String(draft.value.total).replace(',', '.')) * 100)
	 if (!draft.value.title.trim() || !Number.isFinite(totalAmount) || totalAmount <= 0) {
		 toasts.error('Укажите название и сумму')
		 return
	 }
	 creating.value = true
	 try {
		 const cycle = await api.createBillingCycle({
			 title: draft.value.title.trim(), totalAmount,
			 periodStart: unix(draft.value.periodStart), periodEnd: unix(draft.value.periodEnd),
			 paymentDueAt: unix(draft.value.paymentDueAt), graceEndsAt: unix(draft.value.graceEndsAt),
		 })
		 createOpen.value = false
		 await load()
		 await openCycle(cycle)
		 toasts.success('Черновик расчёта создан')
	 } catch (e: any) { toasts.error(e?.message || 'Не удалось создать период') }
	 finally { creating.value = false }
}

async function publish(cycle: BillingCycle) {
	 if (!payerCount.value) { toasts.error('Сначала отметьте хотя бы одного плательщика'); return }
	 if (!window.confirm(`Зафиксировать ${money(cycle.totalAmount)} между ${payerCount.value} плательщиками?`)) return
	 publishing.value = cycle.id
	 try {
		 await api.publishBillingCycle(cycle.id)
		 await load()
		 await openCycle(cycle)
		 toasts.success('Счета опубликованы и больше не пересчитываются')
	 } catch (e: any) { toasts.error(e?.message || 'Не удалось опубликовать') }
	 finally { publishing.value = null }
}

async function markPaid(invoice: BillingInvoice) {
	 paying.value = invoice.id
	 try {
		 await api.markInvoicePaid(invoice.id)
		 if (selected.value) await openCycle(selected.value)
		 await load()
		 toasts.success(`Оплата ${invoice.subscriberName} отмечена`)
	 } catch (e: any) { toasts.error(e?.message || 'Не удалось отметить оплату') }
	 finally { paying.value = null }
}

async function cancelInvoice(invoice: BillingInvoice) {
	 if (!window.confirm(`Списать счёт ${invoice.subscriberName}? Доступ вернётся, если нет других долгов.`)) return
	 canceling.value = invoice.id
	 try {
		 await api.cancelInvoice(invoice.id)
		 if (selected.value) await openCycle(selected.value)
		 await load()
		 toasts.success(`Счёт ${invoice.subscriberName} списан`)
	 } catch (e: any) { toasts.error(e?.message || 'Не удалось списать счёт') }
	 finally { canceling.value = null }
}

async function closeCycle(cycle: BillingCycle) {
	 if (!window.confirm(`Закрыть период «${cycle.title}»? Он уйдёт в архив.`)) return
	 closing.value = cycle.id
	 try {
		 await api.closeBillingCycle(cycle.id)
		 await load()
		 selected.value = null
		 toasts.success('Период закрыт')
	 } catch (e: any) { toasts.error(e?.message || 'Не удалось закрыть период') }
	 finally { closing.value = null }
}

async function deleteDraft(cycle: BillingCycle) {
	 if (!window.confirm(`Удалить черновик «${cycle.title}»? Это нельзя отменить.`)) return
	 deleting.value = cycle.id
	 try {
		 await api.deleteBillingCycle(cycle.id)
		 await load()
		 selected.value = null
		 toasts.success('Черновик удалён')
	 } catch (e: any) { toasts.error(e?.message || 'Не удалось удалить черновик') }
	 finally { deleting.value = null }
}

function invLabel(s: string) { return s === 'paid' ? 'оплачено' : s === 'canceled' ? 'списано' : 'ожидает' }
function invTone(s: string) { return s === 'paid' ? 'success' : s === 'canceled' ? 'neutral' : 'warning' }

onMounted(load)
</script>

<template>
	<div class="min-h-full">
		<TopBar>
			<template #actions>
				<Button variant="secondary" size="sm" @click="createOpen = true">
					<ReceiptText :size="15" /> Новый период
				</Button>
			</template>
		</TopBar>

		<main class="max-w-5xl mx-auto px-4 sm:px-6 pt-10 pb-16 space-y-10">
			<header class="space-y-2 animate-rise">
				<p class="eyebrow">Общие расходы</p>
				<h1 class="num-display text-ink-900 text-[44px] sm:text-[56px]">Хостинг</h1>
				<p class="text-[13.5px] text-ink-500">Сумма делится поровну между плательщиками в момент публикации.</p>
			</header>

			<section class="grid grid-cols-1 sm:grid-cols-3 gap-6 animate-rise delay-1">
				<div><div class="eyebrow mb-2">Получено</div><div class="num-display text-[32px] text-success">{{ money(received) }}</div></div>
				<div><div class="eyebrow mb-2">Ожидается</div><div class="num-display text-[32px] text-warning">{{ money(pending) }}</div></div>
				<div><div class="eyebrow mb-2">Плательщиков сейчас</div><div class="num-display text-[32px] text-ink-900">{{ payerCount }}</div></div>
			</section>

			<section class="space-y-4 animate-rise delay-2">
				<div class="flex items-center gap-4"><h2 class="eyebrow">Расчётные периоды</h2><div class="hairline flex-1" /></div>
				<div v-if="loading" class="card p-6 space-y-3"><Skeleton v-for="i in 3" :key="i" height="52" rounded="lg" /></div>
				<div v-else-if="!cycles.length" class="card p-10 text-center space-y-3">
					<ReceiptText :size="28" class="mx-auto text-ink-300" />
					<p class="font-semibold">Расчётов пока нет</p>
					<p class="text-[12.5px] text-ink-500">Введите стоимость хоста и опубликуйте первый период.</p>
				</div>
				<div v-else class="card divide-y divide-ink-900/5 overflow-hidden">
					<button v-for="cycle in cycles" :key="cycle.id" class="w-full px-5 py-4 flex items-center gap-4 text-left hover:bg-ink-100/50 transition-colors" @click="openCycle(cycle)">
						<CalendarDays :size="18" class="text-ink-400 shrink-0" />
						<div class="flex-1 min-w-0"><div class="font-semibold text-[14px]">{{ cycle.title }}</div><div class="text-[11.5px] text-ink-500 mt-0.5">{{ day(cycle.periodStart) }} — {{ day(cycle.periodEnd) }} · {{ cycle.payerCount || payerCount }} чел.</div></div>
						<div class="text-right"><div class="mono tnum text-[13px] font-semibold">{{ money(cycle.totalAmount) }}</div><Badge :tone="statusTone(cycle.status)" size="xs">{{ cycle.status === 'draft' ? 'черновик' : cycle.status === 'closed' ? 'закрыт' : 'опубликован' }}</Badge></div>
					</button>
				</div>
			</section>
		</main>

		<Modal :open="!!selected" size="lg" :title="selected?.title || 'Расчёт'" @close="selected = null">
			<div v-if="detailLoading" class="space-y-3"><Skeleton v-for="i in 4" :key="i" height="48" rounded="lg" /></div>
			<div v-else-if="selected" class="space-y-5">
				<div class="grid grid-cols-2 gap-3">
					<div class="p-4 rounded-2xl bg-ink-100"><div class="eyebrow mb-1">Сумма</div><div class="num-display text-[25px]">{{ money(selected.totalAmount) }}</div></div>
					<div class="p-4 rounded-2xl bg-ink-100"><div class="eyebrow mb-1">Плательщиков</div><div class="num-display text-[25px]">{{ selected.payerCount || payerCount }}</div></div>
				</div>
				<div class="text-[12px] text-ink-500 flex flex-wrap gap-4"><span><Clock3 :size="13" class="inline" /> оплатить до {{ day(selected.paymentDueAt) }}</span><span>отключение после {{ day(selected.graceEndsAt) }}</span></div>
				<div v-if="selected.status === 'draft'" class="p-4 rounded-2xl bg-warning/10 space-y-3">
					<p class="text-[12.5px] text-warning">При публикации состав и суммы счетов фиксируются навсегда.</p>
					<Button variant="primary" block :loading="publishing === selected.id" @click="publish(selected)"><Users :size="15" /> Опубликовать счета</Button>
					<Button variant="ghost" block :loading="deleting === selected.id" @click="deleteDraft(selected)">Удалить черновик</Button>
				</div>
			<div v-else class="space-y-4">
				<div v-if="selected.status === 'published'" class="flex justify-end">
					<Button variant="ghost" size="sm" :loading="closing === selected.id" @click="closeCycle(selected)">Закрыть период</Button>
				</div>
				<div class="divide-y divide-ink-900/5 rounded-2xl bg-ink-100 overflow-hidden">
					<div v-for="invoice in selected.invoices" :key="invoice.id" class="p-4 flex items-center gap-3">
						<div class="flex-1 min-w-0"><div class="font-semibold text-[13.5px] truncate">{{ invoice.subscriberName }}</div><div class="mono text-[11.5px] text-ink-500">{{ money(invoice.amount) }}</div></div>
						<Badge :tone="invTone(invoice.status)" size="xs"><Check v-if="invoice.status === 'paid'" :size="10" />{{ invLabel(invoice.status) }}</Badge>
						<Button v-if="invoice.status === 'pending'" variant="ghost" size="sm" :loading="paying === invoice.id" @click="markPaid(invoice)">Отметить</Button>
						<Button v-if="invoice.status === 'pending'" variant="ghost" size="sm" :loading="canceling === invoice.id" @click="cancelInvoice(invoice)">Списать</Button>
					</div>
				</div>
			</div>
			</div>
		</Modal>

		<Modal :open="createOpen" size="md" title="Новый расчётный период" @close="createOpen = false">
			<div class="space-y-4">
				<Field label="Название"><Input v-model="draft.title" placeholder="Июль 2026" /></Field>
				<Field label="Стоимость хоста, ₽" :hint="payerCount ? `Примерно ${money(previewShare)} с каждого из ${payerCount}` : 'Нет отмеченных плательщиков'">
					<Input v-model="draft.total" type="number" placeholder="3000" />
				</Field>
				<div class="grid grid-cols-2 gap-3"><Field label="Начало"><Input v-model="draft.periodStart" type="date" /></Field><Field label="Конец"><Input v-model="draft.periodEnd" type="date" /></Field></div>
				<div class="grid grid-cols-2 gap-3"><Field label="Оплатить до"><Input v-model="draft.paymentDueAt" type="date" /></Field><Field label="Отключить после"><Input v-model="draft.graceEndsAt" type="date" /></Field></div>
			</div>
			<template #footer><Button variant="ghost" size="sm" @click="createOpen = false">Отмена</Button><Button variant="primary" size="sm" :loading="creating" @click="createCycle">Создать черновик</Button></template>
		</Modal>
	</div>
</template>
