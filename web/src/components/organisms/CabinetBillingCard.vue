<script setup lang="ts">
import { computed } from 'vue'
import { Check, Clock3, ReceiptText, TriangleAlert } from 'lucide-vue-next'
import type { CabinetBillingSummary } from '@/types'
import Button from '@/components/atoms/Button.vue'
import Badge from '@/components/atoms/Badge.vue'

const props = defineProps<{ billing: CabinetBillingSummary }>()
defineEmits<{ (e: 'pay'): void }>()

const invoice = computed(() => props.billing.latestInvoice)
const cycle = computed(() => props.billing.latestCycle)
const money = computed(() => new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB' }).format((invoice.value?.amount || 0) / 100))
function date(ts?: number) {
	return ts ? new Date(ts * 1000).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' }) : ''
}
const daysLeft = computed(() => cycle.value ? Math.max(0, Math.ceil((cycle.value.graceEndsAt * 1000 - Date.now()) / 86_400_000)) : 0)
</script>

<template>
	<section v-if="billing.billingRole === 'payer'" class="rounded-3xl p-5 mb-5 animate-rise border" :class="{
		'bg-success/8 border-success/15': billing.derivedStatus === 'paid',
		'bg-warning/8 border-warning/15': billing.derivedStatus === 'pending' || billing.derivedStatus === 'grace',
		'bg-danger/8 border-danger/15': billing.derivedStatus === 'overdue',
		'bg-ink-100 border-ink-200': !billing.latestInvoice,
	}">
		<div class="flex items-start gap-3.5">
			<div class="w-10 h-10 rounded-2xl flex items-center justify-center shrink-0" :class="billing.derivedStatus === 'overdue' ? 'bg-danger/12 text-danger' : billing.derivedStatus === 'paid' ? 'bg-success/12 text-success' : 'bg-warning/12 text-warning'">
				<Check v-if="billing.derivedStatus === 'paid'" :size="19" />
				<TriangleAlert v-else-if="billing.derivedStatus === 'overdue'" :size="19" />
				<ReceiptText v-else :size="19" />
			</div>
			<div class="flex-1 min-w-0">
				<div class="flex items-center gap-2 flex-wrap">
					<h2 class="text-[14px] font-semibold text-ink-900">Оплата хостинга</h2>
					<Badge v-if="billing.derivedStatus === 'paid'" tone="success" size="xs">оплачено</Badge>
					<Badge v-else-if="billing.derivedStatus === 'overdue'" tone="danger" size="xs">просрочено</Badge>
					<Badge v-else-if="billing.derivedStatus === 'grace'" tone="warning" size="xs">льготный период</Badge>
				</div>

				<template v-if="invoice && cycle">
					<div class="num-display text-[30px] text-ink-900 mt-2">{{ money }}</div>
					<p v-if="billing.derivedStatus === 'paid'" class="text-[12px] text-ink-500 mt-1">{{ cycle.title }} · VPN активен до {{ date(cycle.periodEnd) }}</p>
					<p v-else-if="billing.derivedStatus === 'overdue'" class="text-[12px] text-danger mt-1">Доступ приостановлен. Устройства сохранятся после оплаты.</p>
					<p v-else-if="billing.derivedStatus === 'grace'" class="text-[12px] text-warning mt-1"><Clock3 :size="12" class="inline" /> До отключения {{ daysLeft }} дн.</p>
					<p v-else class="text-[12px] text-ink-500 mt-1">Оплатить до {{ date(cycle.paymentDueAt) }}</p>
					<Button v-if="billing.derivedStatus !== 'paid' && billing.checkoutEnabled" variant="accent" size="md" block class="mt-4" @click="$emit('pay')">Оплатить через ЮKassa</Button>
					<p v-else-if="billing.derivedStatus !== 'paid'" class="text-[11.5px] text-ink-500 mt-3">Онлайн-оплата пока недоступна. Свяжитесь с администратором.</p>
				</template>
				<p v-else class="text-[12px] text-ink-500 mt-1">На этот кабинет пока не выставлен счёт.</p>
			</div>
		</div>
	</section>
</template>
