<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { api } from '../lib/api';
import type { DataEntry } from '../lib/data-catalog';
import type { BehaviorChecks } from '../lib/workflow';
import { blankBehaviorChecks } from '../lib/behavior-checks';
import ScopeSelect from './ui/ScopeSelect.vue';
const props = defineProps<{ modelValue?: BehaviorChecks; disabled?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [BehaviorChecks] }>();
const entries = ref<DataEntry[]>([]),
  error = ref('');
const c = computed(() => ({
  ...blankBehaviorChecks(),
  ...props.modelValue,
  inputs: props.modelValue?.inputs || {},
  thresholds: props.modelValue?.thresholds || [],
}));
const fields = computed(() =>
  entries.value
    .filter(
      (e) =>
        e.enabled &&
        e.path &&
        ['string', 'number', 'boolean'].includes(e.data_type) &&
        ((e.kind === 'customer_field' && e.path.startsWith('customer.attributes.')) ||
          (e.kind === 'event_field' && e.event_name === c.value.entry_event)),
    )
    .slice(0, 50),
);
const numeric = computed(() =>
  fields.value
    .filter((e) => e.data_type === 'number')
    .map((e) => ({ value: e.path!, label: e.label })),
);
const eventOptions = computed(() => [
  { value: 'all', label: 'Choose the entry event' },
  ...entries.value
    .filter((e) => e.enabled && e.kind === 'event')
    .map((e) => ({ value: e.event_name!, label: e.label })),
]);
const operators = [
  { value: 'greater_than_or_equal', label: 'At least (≥)' },
  { value: 'greater_than', label: 'Greater than (>)' },
  { value: 'less_than_or_equal', label: 'At most (≤)' },
  { value: 'less_than', label: 'Less than (<)' },
];
function patch(data: Partial<BehaviorChecks>) {
  emit('update:modelValue', { ...JSON.parse(JSON.stringify(c.value)), ...data, confirmed: false });
}
function entry(value: string) {
  const event = value === 'all' ? '' : value;
  const inputs: BehaviorChecks['inputs'] = {};
  for (const e of entries.value
    .filter(
      (e) =>
        e.enabled &&
        e.path &&
        ((e.kind === 'customer_field' && e.path.startsWith('customer.attributes.')) ||
          (e.kind === 'event_field' && e.event_name === event)),
    )
    .slice(0, 50)) {
    if (['string', 'number', 'boolean'].includes(e.data_type))
      inputs[e.path!] =
        c.value.inputs[e.path!] ??
        (e.data_type === 'boolean' ? false : e.data_type === 'number' ? 0 : 'Sample');
  }
  patch({ entry_event: event, inputs, thresholds: [] });
}
function numericValue(value: string) {
  return value.trim() === '' ? undefined : Number(value);
}
function count(key: 'email_count' | 'sms_count', event: Event) {
  patch({ [key]: numericValue((event.target as HTMLInputElement).value) });
}
function threshold(index: number, key: 'path' | 'operator' | 'value', value: string) {
  const thresholds: BehaviorChecks['thresholds'] = JSON.parse(JSON.stringify(c.value.thresholds));
  const t = thresholds[index]!;
  if (key === 'value') t.value = Number(value);
  else t[key] = value === 'all' ? '' : value;
  const inputs = { ...c.value.inputs };
  if (t.path && Number.isFinite(t.value)) {
    const delta = Math.max(Math.abs(t.value) * 1e-12, 0.000001);
    inputs[t.path] =
      t.value + (t.operator === 'greater_than' ? delta : t.operator === 'less_than' ? -delta : 0);
  }
  patch({ thresholds, inputs });
}
function inputValue(path: string, type: string, event: Event) {
  const target = event.target as HTMLInputElement;
  const inputs = { ...c.value.inputs };
  if (type === 'number' && target.value === '') delete inputs[path];
  else
    inputs[path] =
      type === 'boolean' ? target.checked : type === 'number' ? Number(target.value) : target.value;
  patch({ inputs });
}
onMounted(async () => {
  try {
    entries.value = await api('/data-catalog');
  } catch {
    error.value = 'Could not load workspace definitions. Reopen this page to retry.';
  }
});
</script>
<template>
  <section class="behavior-checks surface" aria-label="Independent behavior checks">
    <h3>What must be true?</h3>
    <p>
      Define the expected result for one representative customer with both channel consents. Review
      their goal and eligibility values below. These checks are independent of the AI-generated
      steps.
    </p>
    <fieldset :disabled="disabled">
      <div class="behavior-checks-grid">
        <ScopeSelect
          :model-value="c.entry_event || 'all'"
          label="Expected entry"
          :options="eventOptions"
          :disabled="disabled"
          @update:model-value="entry"
        />
        <label
          >Expected emails<input
            type="number"
            min="0"
            max="100"
            :value="c.email_count"
            required
            @input="count('email_count', $event)"
        /></label>
        <label
          >Expected SMS<input
            type="number"
            min="0"
            max="100"
            :value="c.sms_count"
            required
            @input="count('sms_count', $event)"
        /></label>
        <ScopeSelect
          :model-value="c.outcome"
          label="Expected outcome"
          :disabled="disabled"
          :options="[
            { value: 'completed', label: 'Completed without conversion' },
            { value: 'converted', label: 'Goal reached' },
            { value: 'condition_not_met', label: 'Not eligible' },
            { value: 'manually_cancelled', label: 'Cancelled' },
          ]"
          @update:model-value="patch({ outcome: $event as BehaviorChecks['outcome'] })"
        />
        <label
          >Expected duration (hours, optional)<input
            type="number"
            min="0"
            max="8760"
            :value="c.duration_seconds === undefined ? '' : c.duration_seconds / 3600"
            @input="
              patch({
                duration_seconds:
                  numericValue(($event.target as HTMLInputElement).value) === undefined
                    ? undefined
                    : Math.round(Number(($event.target as HTMLInputElement).value) * 3600),
              })
            "
        /></label>
      </div>
      <div class="behavior-thresholds">
        <h4>Eligibility boundaries</h4>
        <p class="help-text">
          Ineligible values must produce no messages or conversion. Adjacent values and the exact
          boundary are tested automatically.
        </p>
        <div v-for="(t, index) in c.thresholds" :key="index" class="behavior-threshold-row">
          <ScopeSelect
            :model-value="t.path || 'all'"
            :label="'Boundary field ' + (index + 1)"
            :options="[{ value: 'all', label: 'Choose a numeric field' }, ...numeric]"
            :disabled="disabled"
            @update:model-value="threshold(index, 'path', $event)"
          />
          <ScopeSelect
            :model-value="t.operator"
            :label="'Boundary comparison ' + (index + 1)"
            :options="operators"
            :disabled="disabled"
            @update:model-value="threshold(index, 'operator', $event)"
          />
          <label
            >Boundary value<input
              type="number"
              step="any"
              :value="t.value"
              @input="threshold(index, 'value', ($event.target as HTMLInputElement).value)"
          /></label>
          <button
            type="button"
            @click="patch({ thresholds: c.thresholds.filter((_, i) => i !== index) })"
          >
            Remove
          </button>
        </div>
        <button
          type="button"
          :disabled="c.thresholds.length >= 4 || !numeric.length"
          @click="
            patch({
              thresholds: [
                ...c.thresholds,
                { path: '', operator: 'greater_than_or_equal', value: 0 },
              ],
            })
          "
        >
          Add boundary
        </button>
      </div>
      <details>
        <summary>Synthetic example values</summary>
        <p>
          Fictional test data only. Goal flags start false and numbers start at zero; eligibility
          boundaries set their example values. Review other qualifying fields here. These values are
          not sent to the model.
        </p>
        <div class="behavior-checks-grid">
          <label v-for="field in fields" :key="field.key"
            >{{ field.label }}
            <input
              v-if="field.data_type === 'boolean'"
              type="checkbox"
              :checked="c.inputs[field.path!] === true"
              @change="inputValue(field.path!, 'boolean', $event)"
            />
            <input
              v-else
              :type="field.data_type === 'number' ? 'number' : 'text'"
              :value="c.inputs[field.path!]"
              maxlength="512"
              step="any"
              @input="inputValue(field.path!, field.data_type, $event)"
            />
          </label>
        </div>
      </details>
      <label class="behavior-confirm"
        ><input
          type="checkbox"
          :checked="c.confirmed"
          @change="
            emit('update:modelValue', {
              ...c,
              confirmed: ($event.target as HTMLInputElement).checked,
            })
          "
        />I reviewed these expectations and the synthetic example.</label
      >
    </fieldset>
    <p v-if="error" role="alert">{{ error }}</p>
  </section>
</template>
