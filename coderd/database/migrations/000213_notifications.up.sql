CREATE TYPE notification_message_status AS ENUM (
    'pending',
    'enqueued',
    'sent',
    'canceled',
    'failed',
    'inhibited',
    'unknown'
    );

CREATE TYPE notification_method AS ENUM (
    'smtp',
    'webhook'
    );

CREATE TABLE notification_templates
(
    id             uuid                 NOT NULL,
    name           text                 NOT NULL,
    enabled        boolean DEFAULT TRUE NOT NULL,
    title_template text                 NOT NULL,
    body_template  text                 NOT NULL,
    "group"        text,
    PRIMARY KEY (id),
    UNIQUE (name)
);

COMMENT ON TABLE notification_templates IS 'Templates from which to create notification messages.';

-- Compute a hash from the template, user, method, input params, targets, and current hour; this will help prevent duplicate
-- messages from being sent within the same hour.
-- It is possible that a message could be sent at 12:59:59 and again at 13:00:00, but this should be good enough for now.
-- This could have been a unique index, but we cannot immutably create an index on a timestamp with a timezone.
-- TODO: md5-hash this?
CREATE OR REPLACE FUNCTION compute_notification_message_dedupe_hash() RETURNS TRIGGER AS
$$
BEGIN
    NEW.dedupe_hash := CONCAT_WS(':',
                                 NEW.notification_template_id,
                                 NEW.user_id,
                                 NEW.method,
                                 NEW.input::text,
                                 ARRAY_TO_STRING(NEW.targets, ','),
                                 DATE_TRUNC('hour', NEW.created_at AT TIME ZONE 'UTC')::text
                       );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION compute_notification_message_dedupe_hash IS 'Computes a unique hash which will be used to prevent duplicate messages from being sent within the last hour';

-- TODO: create a trigger to mark messages inhibited if notification_preferences disables them for a user/org
--          this will need to unnest the targets and check if they intersect with users or orgs

CREATE TABLE notification_messages
(
    id                       uuid                        NOT NULL,
    notification_template_id uuid                        NOT NULL,
    user_id                  uuid                        NOT NULL,
    method                   notification_method         NOT NULL,
    status                   notification_message_status NOT NULL DEFAULT 'pending'::notification_message_status,
    status_reason            text,
    created_by               text                        NOT NULL,
    input                    jsonb                       NOT NULL,
    attempt_count            int                                  DEFAULT 0,
    targets                  uuid[],
    created_at               timestamp with time zone    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at               timestamp with time zone,
    leased_until             timestamp with time zone,
    next_retry_after         timestamp with time zone,
    sent_at                  timestamp with time zone,
    failed_at                timestamp with time zone,
    dedupe_hash              text                        NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (notification_template_id) REFERENCES notification_templates (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    UNIQUE (dedupe_hash)
);

CREATE TRIGGER set_dedupe_hash
    BEFORE INSERT OR UPDATE
    ON notification_messages
    FOR EACH ROW
EXECUTE FUNCTION compute_notification_message_dedupe_hash();

CREATE INDEX idx_notification_messages_status ON notification_messages (status);

COMMENT ON COLUMN notification_messages.dedupe_hash IS 'Auto-generated at insertion time';

CREATE TABLE notification_preferences
(
    id                       uuid NOT NULL,
    notification_template_id uuid NOT NULL,
    disabled                 boolean,
    user_id                  uuid,
    org_id                   uuid,
    PRIMARY KEY (id),
    FOREIGN KEY (notification_template_id) REFERENCES notification_templates (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (org_id) REFERENCES organizations (id) ON DELETE CASCADE
);

-- TODO: autogenerate constants which reference the UUIDs
INSERT INTO notification_templates (id, name, enabled, title_template, body_template, "group")
VALUES ('f517da0b-cdc9-410f-ab89-a86107c420ed', 'Workspace Deleted', true, 'Workspace "{{.name}}" deleted',
        'Hi {{.user_name}}<br>Your workspace <strong>{{.name}}</strong> was deleted.<br>The specified reason was <strong>{{.reason}}</strong>.',
        'Workspace Events');