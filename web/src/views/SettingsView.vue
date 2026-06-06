<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { ServerInfo } from '@/types'
import { api } from '@/lib/api'
import { useToastStore } from '@/stores/toasts'
import TopBar from '@/components/organisms/TopBar.vue'
import Section from '@/components/molecules/Section.vue'
import InfoRow from '@/components/molecules/InfoRow.vue'
import CopyButton from '@/components/molecules/CopyButton.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import Button from '@/components/atoms/Button.vue'
import Spinner from '@/components/atoms/Spinner.vue'

const toasts = useToastStore()
const info = ref<ServerInfo | null>(null)
const loading = ref(true)
const busy = ref<'regen' | 'restart' | 'reset' | null>(null)
const confirmKind = ref<'regen' | 'restart' | 'reset' | null>(null)

async function load() {
  loading.value = true
  try { info.value = await api.serverInfo() }
  catch (e: any) { toasts.error(e?.message || 'Failed to load') }
  finally { loading.value = false }
}
onMounted(load)

const confirmTitle = () => ({
  regen:   'Regenerate H1–H4?',
  restart: 'Restart interface?',
  reset:   'Delete all clients?',
}[confirmKind.value!] || '')

const confirmMessage = () => ({
  regen:   'All existing clients will need to re-import their configurations. The server keys stay the same.',
  restart: 'awg-quick down + up. Active connections drop briefly while the interface comes back. iptables rules reseat automatically.',
  reset:   'Every client will be revoked. This cannot be undone.',
}[confirmKind.value!] || '')

const confirmText = () => ({
  regen:   'Regenerate',
  restart: 'Restart',
  reset:   'Delete all',
}[confirmKind.value!] || '')

async function doConfirm() {
  const k = confirmKind.value
  if (!k) return
  busy.value = k
  confirmKind.value = null
  try {
    if (k === 'regen')   { info.value = await api.regenerateMagic(); toasts.success('Magic regenerated') }
    if (k === 'restart') { await api.restartInterface(); toasts.success('Interface restarted'); await load() }
    if (k === 'reset')   { await api.resetClients();    toasts.success('All clients removed') }
  } catch (e: any) {
    toasts.error(e?.message || 'Action failed')
  } finally {
    busy.value = null
  }
}
</script>

<template>
  <div class="min-h-full">
    <TopBar />

    <main class="max-w-3xl mx-auto px-4 sm:px-5 pt-8 pb-12 space-y-7">
      <div>
        <h1 class="text-[28px] font-semibold text-ink-900 tracking-tight leading-none">Settings</h1>
        <p class="mt-2 text-[13px] text-ink-500">Server identity, obfuscation, and danger-zone actions.</p>
      </div>

      <div v-if="loading" class="card p-10 grid place-items-center"><Spinner :size="22" /></div>

      <template v-else-if="info">
        <Section title="Endpoint">
          <InfoRow label="Public address" :value="info.endpoint" mono show-divider>
            <CopyButton :value="info.endpoint" />
          </InfoRow>
          <InfoRow label="Interface"      :value="info.interface" mono show-divider />
          <InfoRow label="UDP port"       :value="String(info.port)" mono show-divider />
          <InfoRow label="Egress NIC"     :value="info.egressIface" mono />
        </Section>

        <Section title="Identity" footer="The server's public key. Clients use this to authenticate the tunnel.">
          <InfoRow label="Server pubkey" :value="info.publicKey" mono show-divider>
            <CopyButton :value="info.publicKey" />
          </InfoRow>
          <InfoRow label="Server IP" :value="info.address" mono />
        </Section>

        <Section title="Obfuscation" footer="H1–H4 should be random uint32. Regenerating them invalidates every existing client config.">
          <InfoRow label="Jc / Jmin / Jmax" :value="`${info.jc} · ${info.jmin} · ${info.jmax}`" mono show-divider />
          <InfoRow label="S1 / S2"          :value="`${info.s1} · ${info.s2}`" mono show-divider />
          <InfoRow label="H1" :value="info.h1" mono show-divider />
          <InfoRow label="H2" :value="info.h2" mono show-divider />
          <InfoRow label="H3" :value="info.h3" mono show-divider />
          <InfoRow label="H4" :value="info.h4" mono show-divider />
          <InfoRow label="Action">
            <Button size="sm" :loading="busy === 'regen'" @click="confirmKind = 'regen'">Regenerate H1–H4</Button>
          </InfoRow>
        </Section>

        <Section title="Network" footer="These values are baked into every client config we hand out.">
          <InfoRow label="Subnet"               :value="info.subnet" mono show-divider />
          <InfoRow label="DNS"                  :value="info.dns" mono show-divider />
          <InfoRow label="MTU"                  :value="info.mtu ? String(info.mtu) : '— (default)'" mono show-divider />
          <InfoRow label="Allowed IPs"          :value="info.allowedIPs" mono show-divider />
          <InfoRow label="Persistent keepalive" :value="info.persistentKeepalive ? `${info.persistentKeepalive}s` : 'off'" mono />
        </Section>

        <Section title="Danger zone" footer="Both actions affect live traffic. Use the regenerate button above if you only need fresh obfuscation magic.">
          <InfoRow label="Restart interface" show-divider>
            <Button size="sm" :loading="busy === 'restart'" @click="confirmKind = 'restart'">Restart</Button>
          </InfoRow>
          <InfoRow label="Delete all clients">
            <Button variant="danger" size="sm" :loading="busy === 'reset'" @click="confirmKind = 'reset'">Delete all</Button>
          </InfoRow>
        </Section>

        <Section title="Capacity">
          <InfoRow label="Active clients" :value="`${info.clientCount}`" mono />
        </Section>
      </template>
    </main>

    <ConfirmDialog
      :open="confirmKind !== null"
      :title="confirmTitle()"
      :message="confirmMessage()"
      :confirm-text="confirmText()"
      :tone="confirmKind === 'reset' ? 'danger' : 'neutral'"
      @cancel="confirmKind = null"
      @confirm="doConfirm"
    />
  </div>
</template>
