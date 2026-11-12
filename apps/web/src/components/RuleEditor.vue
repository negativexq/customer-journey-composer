<script setup lang="ts">
const operatorLabels: Record<string, string> = {
  equals: 'Equals',
  not_equals: 'Does not equal',
  greater_than: 'Greater than',
  greater_than_or_equal: 'At least (≥)',
  less_than: 'Less than',
  less_than_or_equal: 'At most (≤)',
  contains: 'Contains',
  exists: 'Has a value',
  before: 'Before',
  after: 'After',
};

import type { Rule } from '../lib/workflow';
const props = withDefaults(
  defineProps<{ modelValue?: Rule; readonly?: boolean; depth?: number }>(),
  { depth: 0, modelValue: undefined },
);
const emit = defineEmits<{ 'update:modelValue': [Rule] }>();
const leaf = (): Rule => ({
  path: 'customer.attributes.purchased',
  operator: 'equals',
  value: true,
});
function patch(value: Partial<Rule>) {
  emit('update:modelValue', { ...(props.modelValue || leaf()), ...value });
}
function value(text: string) {
  try {
    patch({ value: JSON.parse(text) });
  } catch {
    patch({ value: text });
  }
}
function group() {
  emit('update:modelValue', { logic: 'and', rules: [props.modelValue || leaf(), leaf()] });
}
function child(index: number, rule: Rule) {
  const rules = [...(props.modelValue?.rules || [])];
  rules[index] = rule;
  patch({ rules });
}
</script>
<template>
  <div class="rule-editor">
    <template v-if="modelValue?.rules"
      ><label
        >Match<select
          :value="modelValue.logic"
          :disabled="readonly"
          @change="patch({ logic: ($event.target as HTMLSelectElement).value as 'and' | 'or' })"
        >
          <option value="and">All conditions (AND)</option>
          <option value="or">Any condition (OR)</option>
        </select></label
      >
      <div v-for="(rule, index) in modelValue.rules" :key="index">
        <RuleEditor
          :model-value="rule"
          :readonly="readonly"
          :depth="depth + 1"
          @update:model-value="child(index, $event)"
        /><button
          v-if="!readonly && modelValue.rules.length > 1"
          @click="patch({ rules: modelValue.rules?.filter((_, i) => i !== index) })"
        >
          Remove condition
        </button>
      </div>
      <button v-if="!readonly" @click="patch({ rules: [...modelValue.rules, leaf()] })">
        Add condition
      </button></template
    ><template v-else
      ><label
        >Field<input
          :value="modelValue?.path"
          :disabled="readonly"
          placeholder="customer.attributes.total_orders"
          @input="patch({ path: ($event.target as HTMLInputElement).value })" /></label
      ><label
        >Operator<select
          :value="modelValue?.operator || 'equals'"
          :disabled="readonly"
          @change="patch({ operator: ($event.target as HTMLSelectElement).value })"
        >
          <option
            v-for="operator in [
              'equals',
              'not_equals',
              'greater_than',
              'greater_than_or_equal',
              'less_than',
              'less_than_or_equal',
              'contains',
              'exists',
              'before',
              'after',
            ]"
            :key="operator"
          >
            {{ operatorLabels[operator] || operator }}
          </option>
        </select></label
      ><label v-if="modelValue?.operator !== 'exists'"
        >Value<input
          :value="
            typeof modelValue?.value === 'string'
              ? modelValue.value
              : JSON.stringify(modelValue?.value)
          "
          :disabled="readonly"
          @change="value(($event.target as HTMLInputElement).value)" /></label
      ><button v-if="!readonly && depth < 7" @click="group">Add AND / OR group</button></template
    >
  </div>
</template>
