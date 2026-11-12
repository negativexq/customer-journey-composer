<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { api } from '../lib/api';
import type { DataEntry } from '../lib/data-catalog';
import type { BehaviorChecks, BehaviorCase, BehaviorEvent, Rule } from '../lib/workflow';
import { blankBehaviorChecks } from '../lib/behavior-checks';
import ScopeSelect from './ui/ScopeSelect.vue';
const props = defineProps<{
  modelValue?: BehaviorChecks;
  disabled?: boolean;
  embedded?: boolean;
  catalog?: DataEntry[];
}>();
const emit = defineEmits<{ 'update:modelValue': [BehaviorChecks] }>();
const entries = ref<DataEntry[]>([]),
  error = ref('');
const c = computed(() => ({
  ...blankBehaviorChecks(),
  ...props.modelValue,
  inputs: props.modelValue?.inputs || {},
  thresholds: props.modelValue?.thresholds || [],
  case_name:
    props.modelValue?.case_name || (props.modelValue?.version === 1 ? '' : 'Default situation'),
  consents:
    props.modelValue?.consents ||
    (props.modelValue?.version === 1 ? {} : { email: true, sms: true }),
  ineligible_outcome:
    props.modelValue?.version === 1
      ? props.modelValue?.ineligible_outcome
      : props.modelValue?.ineligible_outcome || 'condition_not_met',
  conditions: props.modelValue?.conditions || [],
  events: props.modelValue?.events || [],
  alternatives: props.modelValue?.alternatives || [],
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
watch(
  () => props.catalog,
  (value) => {
    if (value) entries.value = value;
  },
);
function clauseValue(a: BehaviorCase): BehaviorChecks {
  return {
    ...blankBehaviorChecks(),
    ...a,
    case_name: a.name,
    entry_event: c.value.entry_event,
    thresholds: [],
    alternatives: [],
    confirmed: false,
  };
}
function updateClause(index: number, next: BehaviorChecks) {
  const alternatives = [...c.value.alternatives];
  alternatives[index] = {
    name: next.case_name || '',
    conditions: next.conditions,
    inputs: next.inputs,
    consents: next.consents,
    events: next.events,
    email_count: next.email_count,
    sms_count: next.sms_count,
    outcome: next.outcome,
    duration_seconds: next.duration_seconds,
    expected_attributes: next.expected_attributes,
  };
  patch({ alternatives });
}
function scope(index: number, change: Partial<Rule>) {
  const conditions: Rule[] = JSON.parse(JSON.stringify(c.value.conditions));
  const r = conditions[index]!;
  Object.assign(r, change);
  if (change.path) {
    const f = fields.value.find((f) => f.path === change.path);
    r.value = f?.data_type === 'boolean' ? false : f?.data_type === 'number' ? 0 : '';
    r.operator = 'equals';
  }
  const inputs = { ...c.value.inputs };
  if (r.path) {
    const n = Number(r.value);
    inputs[r.path] = [
      'greater_than',
      'greater_than_or_equal',
      'less_than',
      'less_than_or_equal',
    ].includes(r.operator || '')
      ? n + (r.operator === 'greater_than' ? 0.000001 : r.operator === 'less_than' ? -0.000001 : 0)
      : (r.value as string | number | boolean | null);
  }
  patch({ conditions, inputs });
}
function scopedValue(index: number, event: Event) {
  const r = c.value.conditions[index]!,
    f = fields.value.find((f) => f.path === r.path),
    target = event.target as HTMLInputElement;
  scope(index, {
    value:
      f?.data_type === 'boolean'
        ? target.checked
        : f?.data_type === 'number'
          ? Number(target.value)
          : target.value,
  });
}
function timeline(index: number, key: 'event_name' | 'after_seconds', value: string) {
  const events = JSON.parse(JSON.stringify(c.value.events));
  events[index][key] =
    key === 'after_seconds' ? Math.round(Number(value) * 3600) : value === 'all' ? '' : value;
  patch({ events });
}
function binding(index: number, change: Partial<NonNullable<BehaviorEvent['entry_binding']>>) {
  const events = JSON.parse(JSON.stringify(c.value.events));
  events[index].entry_binding = {
    entry_path: '',
    event_path: '',
    operator: 'equals',
    ...events[index].entry_binding,
    ...change,
  };
  patch({ events });
}
function eventProperty(index: number, path: string, type: string, event: Event) {
  const events = JSON.parse(JSON.stringify(c.value.events)),
    target = event.target as HTMLInputElement;
  events[index].properties ||= {};
  events[index].properties[path] =
    type === 'boolean' ? target.checked : type === 'number' ? Number(target.value) : target.value;
  patch({ events });
}
onMounted(async () => {
  try {
    entries.value = props.catalog || (await api('/data-catalog'));
  } catch {
    error.value = 'Could not load workspace definitions. Reopen this page to retry.';
  }
});
</script>
<template>
  <section
    class="behavior-checks surface"
    :aria-label="embedded ? 'Alternative contract clause' : 'Independent behavior checks'"
  >
    <h3>{{ embedded ? 'Alternative situation' : 'What must be true?' }}</h3>
    <p>
      Each expected result belongs to the situation and event sequence below. Add alternative
      situations for different business outcomes. These obligations are independent of the generated
      graph.
    </p>
    <fieldset :disabled="disabled">
      <button
        v-if="c.version === 1"
        type="button"
        @click="
          patch({
            version: 2,
            case_name: 'Default situation',
            ineligible_outcome: 'condition_not_met',
          })
        "
      >
        Upgrade to scoped contract
      </button>
      <template v-if="c.version === 2">
        <label
          >Situation name<input
            :value="c.case_name"
            maxlength="160"
            @input="patch({ case_name: ($event.target as HTMLInputElement).value })"
        /></label>
        <p class="help-text">
          Conditions and event timing are shared with the model as business rules. Private example
          values and event payloads stay on the server.
        </p>
        <div class="behavior-checks-grid">
          <label v-for="channel in ['email', 'sms']" :key="channel"
            ><input
              type="checkbox"
              :checked="c.consents?.[channel] ?? true"
              @change="
                patch({
                  consents: {
                    ...c.consents,
                    [channel]: ($event.target as HTMLInputElement).checked,
                  },
                })
              "
            />{{ channel === 'email' ? 'Email' : 'SMS' }} consent in this situation</label
          >
        </div>
        <div v-for="(r, index) in c.conditions" :key="index" class="behavior-threshold-row">
          <ScopeSelect
            :model-value="r.path || 'all'"
            :label="'Situation field ' + (index + 1)"
            :options="[
              { value: 'all', label: 'Choose a business field' },
              ...fields.map((f) => ({ value: f.path!, label: f.label })),
            ]"
            :disabled="disabled"
            @update:model-value="scope(index, { path: $event === 'all' ? '' : $event })"
          />
          <ScopeSelect
            :model-value="r.operator || 'equals'"
            :label="'Situation comparison ' + (index + 1)"
            :options="[{ value: 'equals', label: 'Equals' }, ...operators]"
            :disabled="disabled"
            @update:model-value="scope(index, { operator: $event })"
          />
          <label
            >Situation value<input
              :type="
                fields.find((f) => f.path === r.path)?.data_type === 'boolean'
                  ? 'checkbox'
                  : fields.find((f) => f.path === r.path)?.data_type === 'number'
                    ? 'number'
                    : 'text'
              "
              :value="r.value"
              :checked="r.value === true"
              step="any"
              maxlength="512"
              @input="scopedValue(index, $event)"
          /></label>
          <button
            type="button"
            @click="patch({ conditions: c.conditions.filter((_, i) => i !== index) })"
          >
            Remove condition
          </button>
        </div>
        <button
          type="button"
          :disabled="c.conditions.length >= 8 || !fields.length"
          @click="
            patch({ conditions: [...c.conditions, { path: '', operator: 'equals', value: false }] })
          "
        >
          Add initial condition
        </button>
        <h4>Subsequent events</h4>
        <p class="help-text">
          No events means none arrive before the journey completes or times out. It does not forbid
          messages on other paths.
        </p>
        <div v-for="(e, index) in c.events" :key="index" class="contract-event">
          <div class="behavior-checks-grid">
            <ScopeSelect
              :model-value="e.event_name || 'all'"
              :label="'Follow-up event ' + (index + 1)"
              :options="eventOptions"
              :disabled="disabled"
              @update:model-value="timeline(index, 'event_name', $event)"
            />
            <label
              >Hours after entry<input
                type="number"
                min="0"
                max="8760"
                step="any"
                :value="e.after_seconds / 3600"
                @input="
                  timeline(index, 'after_seconds', ($event.target as HTMLInputElement).value)
                "
            /></label>
          </div>
          <div v-if="e.entry_binding" class="behavior-checks-grid">
            <ScopeSelect
              :model-value="e.entry_binding.entry_path || 'all'"
              label="Entry identifier"
              :options="[
                { value: 'all', label: 'Choose entry identifier' },
                ...entries
                  .filter(
                    (f) =>
                      f.enabled &&
                      f.kind === 'event_field' &&
                      f.event_name === c.entry_event &&
                      f.path &&
                      ['string', 'number'].includes(f.data_type),
                  )
                  .map((f) => ({ value: f.path!, label: f.label })),
              ]"
              :disabled="disabled"
              @update:model-value="binding(index, { entry_path: $event === 'all' ? '' : $event })"
            />
            <ScopeSelect
              :model-value="e.entry_binding.event_path || 'all'"
              label="Event identifier"
              :options="[
                { value: 'all', label: 'Choose event identifier' },
                ...entries
                  .filter(
                    (f) =>
                      f.enabled &&
                      f.kind === 'event_field' &&
                      f.event_name === e.event_name &&
                      f.path &&
                      ['string', 'number'].includes(f.data_type),
                  )
                  .map((f) => ({ value: f.path!, label: f.label })),
              ]"
              :disabled="disabled"
              @update:model-value="binding(index, { event_path: $event === 'all' ? '' : $event })"
            />
            <ScopeSelect
              :model-value="e.entry_binding.operator"
              label="Event relationship"
              :options="[
                { value: 'equals', label: 'Same object as entry' },
                { value: 'not_equals', label: 'Different object from entry' },
              ]"
              :disabled="disabled"
              @update:model-value="binding(index, { operator: $event as 'equals' | 'not_equals' })"
            />
          </div>
          <button v-if="!e.entry_binding" type="button" @click="binding(index, {})">
            Define event identity scope
          </button>
          <button
            v-else
            type="button"
            @click="
              patch({
                events: c.events.map((event, i) =>
                  i === index ? { ...event, entry_binding: undefined } : event,
                ),
              })
            "
          >
            Remove event identity scope
          </button>
          <details>
            <summary>Private event example</summary>
            <div class="behavior-checks-grid">
              <label
                v-for="f in entries.filter(
                  (f) =>
                    f.enabled &&
                    f.kind === 'event_field' &&
                    f.event_name === e.event_name &&
                    f.path,
                )"
                :key="f.key"
                >{{ f.label
                }}<input
                  :type="
                    f.data_type === 'boolean'
                      ? 'checkbox'
                      : f.data_type === 'number'
                        ? 'number'
                        : 'text'
                  "
                  :value="e.properties?.[f.path!.replace('event.properties.', '')]"
                  :checked="e.properties?.[f.path!.replace('event.properties.', '')] === true"
                  step="any"
                  maxlength="512"
                  @input="
                    eventProperty(
                      index,
                      f.path!.replace('event.properties.', ''),
                      f.data_type,
                      $event,
                    )
                  "
              /></label>
            </div>
          </details>
          <button type="button" @click="patch({ events: c.events.filter((_, i) => i !== index) })">
            Remove event
          </button>
        </div>
        <button
          type="button"
          :disabled="c.events.length >= 8"
          @click="
            patch({
              events: [
                ...c.events,
                { event_name: '', after_seconds: c.events.at(-1)?.after_seconds || 0 },
              ],
            })
          "
        >
          Add subsequent event
        </button>
      </template>
      <div class="behavior-checks-grid">
        <ScopeSelect
          v-if="!embedded"
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
      <div v-if="!embedded" class="behavior-thresholds">
        <h4>Eligibility boundaries</h4>
        <p class="help-text">
          These are eligibility rules, not examples. Ineligible inputs must not send messages or
          change attributes; their exit reason is specified separately. Exact and adjacent values
          are checked.
        </p>
        <ScopeSelect
          v-if="c.version === 2 && c.thresholds.length"
          :model-value="c.ineligible_outcome || 'condition_not_met'"
          label="Ineligible outcome"
          :disabled="disabled"
          :options="[
            { value: 'condition_not_met', label: 'Not eligible' },
            { value: 'completed', label: 'Completed without conversion' },
            { value: 'manually_cancelled', label: 'Cancelled' },
          ]"
          @update:model-value="
            patch({ ineligible_outcome: $event as BehaviorChecks['ineligible_outcome'] })
          "
        />
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
          boundaries set their example values. Review other qualifying fields here. These example
          fields are excluded from provider input; initial conditions above are separate, explicitly
          shared business rules.
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
      <template v-if="!embedded && c.version === 2">
        <section
          v-for="(alternative, index) in c.alternatives"
          :key="index"
          class="contract-alternative"
        >
          <BehaviorChecksEditor
            :model-value="clauseValue(alternative)"
            :embedded="true"
            :catalog="entries"
            :disabled="disabled"
            @update:model-value="updateClause(index, $event)"
          />
          <button
            type="button"
            @click="patch({ alternatives: c.alternatives.filter((_, i) => i !== index) })"
          >
            Remove alternative situation
          </button>
        </section>
        <button
          type="button"
          :disabled="c.alternatives.length >= 7"
          @click="
            patch({
              alternatives: [
                ...c.alternatives,
                {
                  name: 'Alternative situation',
                  inputs: JSON.parse(JSON.stringify(c.inputs)),
                  conditions: [],
                  events: [],
                  consents: { ...c.consents },
                  email_count: undefined,
                  sms_count: undefined,
                  outcome: 'completed',
                },
              ],
            })
          "
        >
          Add alternative situation
        </button>
      </template>
      <label v-if="!embedded" class="behavior-confirm"
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
