-- Push notifications are a third message channel. A customer is reachable by
-- push when the profile holds a device token and push consent; the demo sends
-- through a mock provider, so no real device is ever contacted.
ALTER TABLE customers ADD COLUMN push_token text CHECK (push_token IS NULL OR char_length(push_token) BETWEEN 8 AND 512);

ALTER TABLE action_outbox DROP CONSTRAINT action_outbox_channel_check;
ALTER TABLE action_outbox ADD CONSTRAINT action_outbox_channel_check CHECK (channel IN ('email','sms','push'));
ALTER TABLE message_templates DROP CONSTRAINT message_templates_channel_check;
ALTER TABLE message_templates ADD CONSTRAINT message_templates_channel_check CHECK (channel IN ('email','sms','push'));
ALTER TABLE delivery_jobs DROP CONSTRAINT delivery_jobs_channel_check;
ALTER TABLE delivery_jobs ADD CONSTRAINT delivery_jobs_channel_check CHECK (channel IN ('email','sms','push'));
ALTER TABLE delivery_jobs DROP CONSTRAINT delivery_jobs_provider_check;
ALTER TABLE delivery_jobs ADD CONSTRAINT delivery_jobs_provider_check CHECK (provider IN ('smtp','mock_sms','mock_email','mock_push'));
ALTER TABLE test_messages DROP CONSTRAINT test_messages_channel_check;
ALTER TABLE test_messages ADD CONSTRAINT test_messages_channel_check CHECK (channel IN ('email','sms','push'));
ALTER TABLE provider_channels DROP CONSTRAINT provider_channels_provider_check;
ALTER TABLE provider_channels ADD CONSTRAINT provider_channels_provider_check CHECK (provider IN ('smtp','mock_sms','mock_email','mock_push'));
