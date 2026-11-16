<script setup lang="ts">
import AppSelect from './ui/AppSelect.vue';
import { errorMessage, plural } from '../lib/format';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import CustomerProfileFields from './ui/CustomerProfileFields.vue';
import { api } from '../lib/api';
import type { Config } from '../lib/workflow';
const props = defineProps<{ modelValue: Config; channel: 'email' | 'sms'; readonly?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [Config] }>();
type Message = {
  channel: string;
  to: string;
  subject: string;
  preheader: string;
  html: string;
  text: string;
  personalization_values: Record<string, string>;
  missing_attributes: string[];
  links: string[];
  warnings: string[];
  encoding: string;
  units: number;
  segments: number;
  longest_case?: {
    values: Record<string, string>;
    encoding: string;
    units: number;
    segments: number;
    tracked_links: number;
  };
};
const templates = ref<{ id: string; name: string; channel: string }[]>([]);
const templateID = ref('');
const templateName = ref('');
const profile = ref('Custom Test Customer');
const customerID = ref('');
const customer = ref(
  JSON.stringify(
    {
      email: 'preview@example.invalid',
      phone: '+905551234567',
      locale: 'en',
      timezone: 'UTC',
      attributes: { first_name: 'Ada', total_orders: 0, cart_amount: 2450 },
      consents: { email: true, sms: true },
    },
    null,
    2,
  ),
);
const event = ref('{"properties":{"cart_id":"cart_demo","amount":2450,"currency":"TRY"}}');
const preview = ref<Message>();
const error = ref('');
const notice = ref('');
const loading = ref(false);
const device = ref('desktop');
const recipient = ref('');
const testKey = ref(crypto.randomUUID());
const sending = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;
let poll: ReturnType<typeof setTimeout> | undefined;
let sequence = 0;
let disposed = false;
const escapeText = (text: string) =>
  text.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const document = computed(
  () =>
    `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'"><meta name="viewport" content="width=device-width"><style>body{font-family:Arial,sans-serif;padding:24px;color:#29362a;line-height:1.7}a{color:#365b35}img{max-width:100%}</style></head><body>${preview.value?.html || '<pre>' + escapeText(preview.value?.text || '') + '</pre>'}</body></html>`,
);
function patch(values: Partial<Config>) {
  emit('update:modelValue', { ...props.modelValue, ...values });
}
function payload() {
  return {
    channel: props.channel,
    config: props.modelValue,
    customer_id: customerID.value || undefined,
    customer: JSON.parse(customer.value),
    event: JSON.parse(event.value),
  };
}
async function render() {
  const request = ++sequence;
  error.value = '';
  loading.value = true;
  try {
    const result = await api<Message>('/messages/preview', 'POST', payload());
    if (request === sequence && !disposed) preview.value = result;
  } catch (cause) {
    if (request === sequence) error.value = errorMessage(cause);
  } finally {
    if (request === sequence) loading.value = false;
  }
}
watch(
  [() => props.modelValue, customer, event, customerID, () => props.channel],
  () => {
    clearTimeout(timer);
    testKey.value = crypto.randomUUID();
    timer = setTimeout(() => void render(), 450);
  },
  { deep: true },
);
watch(recipient, () => (testKey.value = crypto.randomUUID()));
function chooseProfile() {
  if (profile.value === 'Existing Customer') {
    customer.value = '{}';
    return;
  }
  customerID.value = '';
  const orders = profile.value === 'Returning Customer' ? 4 : 0;
  customer.value = JSON.stringify(
    {
      email: 'preview@example.invalid',
      phone: '+905551234567',
      locale: 'en',
      timezone: 'UTC',
      attributes: {
        first_name: 'Ada',
        total_orders: orders,
        cart_amount: 2450,
        ...(profile.value === 'Inactive Customer'
          ? { last_seen_at: new Date(Date.now() - 90 * 86400000).toISOString() }
          : {}),
      },
      consents: { email: true, sms: true },
    },
    null,
    2,
  );
}
async function loadTemplates() {
  try {
    templates.value = await api('/message-templates');
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
async function useTemplate() {
  if (!templateID.value) return;
  try {
    const result = await api<{ config: Config }>(`/message-templates/${templateID.value}`);
    patch({ ...result.config, template_id: templateID.value });
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
const codeVariable = '{{ offer.code }}';
const showsCode = computed(() =>
  [
    props.modelValue.subject,
    props.modelValue.preheader,
    props.modelValue.html,
    props.modelValue.text,
  ]
    .filter(Boolean)
    .some((content) => /\{\{\s*offer\.code\s*\}\}/.test(content as string)),
);
// A code a customer types at checkout: capitals, digits and hyphens.
function setCode(value: string) {
  const code = value
    .toUpperCase()
    .replace(/[^A-Z0-9-]/g, '')
    .slice(0, 30);
  patch({ discount_code: code || undefined });
}
const codeHelp = computed(() => {
  if (props.channel === 'email')
    return showsCode.value
      ? `The body shows the code where it writes ${codeVariable}.`
      : 'The email shows the code in a box of its own below the body.';
  return showsCode.value
    ? `Filled in where the SMS writes ${codeVariable}, so it cannot be mistyped.`
    : `An SMS must show its code: add ${codeVariable} to the message.`;
});
function insertCode() {
  const text = props.modelValue.text || '';
  patch({ text: text + (text && !text.endsWith(' ') ? ' ' : '') + codeVariable });
}
const longestNote = computed(() => {
  const worst = preview.value?.longest_case;
  if (!worst || props.channel !== 'sms') return '';
  const names = Object.entries(worst.values)
    .map(([path, value]) => `${path.replace('customer.attributes.', '')} “${value}”`)
    .join(', ');
  const parts = [
    `${worst.segments} segment${worst.segments === 1 ? '' : 's'} (${worst.encoding}, ${worst.units} units)`,
    names ? `with ${names}, the costliest value among customers who accept SMS` : '',
    worst.tracked_links
      ? `${worst.tracked_links} link${worst.tracked_links === 1 ? '' : 's'} counted at tracked length`
      : '',
  ].filter(Boolean);
  return 'As sent: ' + parts.join(', ');
});
function setFallbacks(value: string) {
  try {
    patch({ fallbacks: JSON.parse(value) });
    error.value = '';
  } catch {
    error.value = 'Fallbacks must be valid JSON';
  }
}
async function saveTemplate() {
  try {
    await api('/message-templates', 'POST', {
      name: templateName.value,
      channel: props.channel,
      config: props.modelValue,
    });
    notice.value = 'Template saved';
    templateName.value = '';
    await loadTemplates();
  } catch (cause) {
    error.value = errorMessage(cause);
  }
}
async function pollDelivery(id: string) {
  if (disposed) return;
  try {
    const job = await api<{ status: string; last_error?: string }>(`/deliveries/${id}`);
    notice.value = `Test ${job.status.toLowerCase()}${job.last_error ? `: ${job.last_error}` : ''}`;
    if (['SCHEDULED', 'CLAIMED', 'DISPATCHING'].includes(job.status))
      poll = setTimeout(() => void pollDelivery(id), 1000);
    else sending.value = false;
  } catch (cause) {
    error.value = errorMessage(cause);
    sending.value = false;
  }
}
async function sendTest() {
  sending.value = true;
  error.value = '';
  try {
    const job = await api<{ id: string }>(
      '/messages/test-send',
      'POST',
      { preview: payload(), recipient: recipient.value },
      { 'Idempotency-Key': testKey.value },
    );
    notice.value = 'Test scheduled';
    await pollDelivery(job.id);
  } catch (cause) {
    error.value = errorMessage(cause);
    sending.value = false;
  }
}
async function copy() {
  try {
    await navigator.clipboard.writeText(preview.value?.html || preview.value?.text || '');
    notice.value = 'Rendered content copied';
  } catch {
    error.value = 'Clipboard is unavailable in this browser.';
  }
}
onMounted(() => {
  void render();
  void loadTemplates();
});
onBeforeUnmount(() => {
  disposed = true;
  clearTimeout(timer);
  clearTimeout(poll);
});
</script>
<template>
  <div class="channel-studio">
    <section class="panel editor-form">
      <div class="card-top">
        <h2>{{ channel === 'email' ? 'Email studio' : 'SMS studio' }}</h2>
        <span class="pill">{{ channel.toUpperCase() }}</span>
      </div>
      <label
        >Message template<AppSelect
          v-model="templateID"
          :disabled="readonly"
          :options="[
            { value: '', label: 'Write my own', description: 'Edit the subject and body below' },
            ...templates
              .filter((t) => t.channel === channel)
              .map((template) => ({ value: template.id, label: template.name })),
          ]"
          @change="useTemplate" /></label
      ><template v-if="channel === 'email'"
        ><label
          >Subject<input
            :value="modelValue.subject"
            :disabled="readonly"
            maxlength="998"
            @input="patch({ subject: ($event.target as HTMLInputElement).value })" /></label
        ><label
          >Preheader<input
            :value="modelValue.preheader"
            :disabled="readonly"
            maxlength="300"
            @input="patch({ preheader: ($event.target as HTMLInputElement).value })" /></label
        ><label
          >HTML body<textarea
            :value="modelValue.html"
            :disabled="readonly"
            rows="9"
            class="code-input"
            @input="patch({ html: ($event.target as HTMLTextAreaElement).value })"
          /></label></template
      ><label
        >{{ channel === 'email' ? 'Text fallback' : 'SMS message'
        }}<textarea
          :value="modelValue.text"
          :disabled="readonly"
          rows="5"
          @input="patch({ text: ($event.target as HTMLTextAreaElement).value })"
        /></label
      ><small
        >Use variables such as &#123;&#123; customer.attributes.first_name &#125;&#125;. Values are
        rendered safely; no code or expressions run.</small
      >
      <div class="discount-field">
        <label
          >Discount code<input
            :value="modelValue.discount_code || ''"
            :disabled="readonly"
            maxlength="30"
            placeholder="e.g. SEPET15"
            autocapitalize="characters"
            spellcheck="false"
            @input="setCode(($event.target as HTMLInputElement).value)" /></label
        ><button
          v-if="!readonly && channel === 'sms' && modelValue.discount_code && !showsCode"
          type="button"
          @click="insertCode"
        >
          Add code to message
        </button>
      </div>
      <small v-if="modelValue.discount_code">{{ codeHelp }}</small>
      <details>
        <summary>Missing-value fallbacks</summary>
        <label
          >Fallback values (JSON)<textarea
            :value="JSON.stringify(modelValue.fallbacks || {}, null, 2)"
            rows="4"
            @change="setFallbacks(($event.target as HTMLTextAreaElement).value)"
          />
        </label>
      </details>
      <form v-if="!readonly" class="inline-form" @submit.prevent="saveTemplate">
        <label>Save as template<input v-model="templateName" required maxlength="160" /></label
        ><button>Save template</button>
      </form>
    </section>
    <section class="preview-studio">
      <div class="panel editor-form">
        <h2>Preview as customer</h2>
        <label
          >Profile<AppSelect
            v-model="profile"
            :options="[
              {
                value: 'Existing Customer',
                label: 'Existing customer',
                description: 'Copy a profile from this workspace',
              },
              {
                value: 'New Customer',
                label: 'New customer',
                description: 'Signed up today, no orders',
              },
              {
                value: 'Returning Customer',
                label: 'Returning customer',
                description: 'Has ordered before',
              },
              {
                value: 'Inactive Customer',
                label: 'Inactive customer',
                description: 'No activity for months',
              },
              {
                value: 'Custom Test Customer',
                label: 'Custom',
                description: 'Edit every field yourself',
              },
            ]"
            @change="chooseProfile" /></label
        ><label v-if="profile === 'Existing Customer'"
          >Existing customer ID<input
            v-model="customerID"
            placeholder="Customer ID from this workspace"
        /></label>
        <CustomerProfileFields v-model="customer" />
        <details>
          <summary>Customize preview data</summary>
          <label
            >Customer overrides (JSON)<textarea
              v-model="customer"
              rows="10"
              class="code-input"
            /></label
          ><label
            >Event context (JSON)<textarea v-model="event" rows="5" class="code-input" /></label
          ><small>Overrides are isolated. The saved customer profile is never modified.</small>
        </details>
        <div class="toolbar">
          <button :disabled="loading" @click="render">Refresh preview</button
          ><button
            v-if="channel === 'email'"
            @click="device = device === 'desktop' ? 'mobile' : 'desktop'"
          >
            {{ device === 'desktop' ? 'Mobile preview' : 'Desktop preview' }}</button
          ><button :disabled="!preview" @click="copy">Copy rendered content</button>
        </div>
      </div>
      <p v-if="error" role="alert" class="error-message">{{ error }}</p>
      <p role="status">{{ notice || (loading ? 'Rendering preview…' : '') }}</p>
      <div v-if="preview" class="panel rendered-message">
        <template v-if="channel === 'email'"
          ><h3>{{ preview.subject }}</h3>
          <p>{{ preview.preheader }}</p>
          <div class="email-frame" :class="device">
            <iframe
              title="Isolated email preview"
              :srcdoc="document"
              sandbox=""
              referrerpolicy="no-referrer"
            /></div></template
        ><template v-else
          ><div class="phone-preview">
            <small>MESSAGES · NOW</small>
            <div class="sms-bubble">{{ preview.text }}</div>
          </div>
          <p>
            {{ preview.encoding }} · {{ preview.units }} units ·
            {{ plural(preview.segments, 'estimated segment') }}
          </p>
          <p
            v-if="longestNote"
            :class="
              (preview.longest_case?.segments ?? 0) > preview.segments
                ? 'warning-message'
                : 'longest-note'
            "
          >
            {{ longestNote }}
          </p></template
        >
        <p v-if="preview.missing_attributes.length" class="error-message">
          Missing attributes: {{ preview.missing_attributes.join(', ') }}
        </p>
        <p v-for="warning in preview.warnings" :key="warning" class="warning-message">
          {{ warning }}
        </p>
        <details>
          <summary>Personalization values & CTA links</summary>
          <pre>{{ JSON.stringify(preview.personalization_values, null, 2) }}</pre>
          <p v-for="link in preview.links" :key="link">{{ link }}</p>
        </details>
      </div>
      <div v-if="!readonly" class="panel editor-form">
        <h3>Send a test {{ channel === 'email' ? 'email' : 'SMS' }}</h3>
        <p>
          Use an allowlisted test recipient. Email uses SMTP; SMS uses the mock provider. This is
          separate from live journey execution.
        </p>
        <label
          >Test recipient<input
            v-model="recipient"
            :type="channel === 'email' ? 'email' : 'tel'"
            required /></label
        ><button
          class="primary"
          :disabled="sending || !recipient || !!preview?.missing_attributes.length"
          @click="sendTest"
        >
          {{ sending ? 'Sending test…' : `Send test ${channel === 'email' ? 'email' : 'SMS'}` }}
        </button>
      </div>
    </section>
  </div>
</template>
