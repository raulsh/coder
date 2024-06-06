-- name: InsertNotificationTemplate :one
INSERT INTO notification_templates (id, name, enabled, title_template, body_template, "group")
VALUES ($1,
        $2,
        $3,
        $4,
        $5,
        $6)
RETURNING *;

-- name: EnqueueNotificationMessage :one
INSERT INTO notification_messages (id, notification_template_id, user_id, method, input, targets, created_by)
VALUES (@id,
        @notification_template_id,
        @user_id,
        @method::notification_method,
        @input::jsonb,
        @targets,
        @created_by)
RETURNING *;

-- Acquires the lease for a given count of notification messages, to enable concurrent dequeuing and subsequent sending.
-- Only rows that aren't already leased (or ones which are leased but have exceeded their lease period) are returned.
-- "Lease" here refers to marking the row as 'leased'.
--
-- SKIP LOCKED is used to jump over locked rows. This prevents
-- multiple notifiers from acquiring the same messages. See:
-- https://www.postgresql.org/docs/9.5/sql-select.html#SQL-FOR-UPDATE-SHARE
-- name: AcquireNotificationMessages :many
WITH acquired AS (
    UPDATE
        notification_messages
            SET updated_at = NOW(),
                status = 'leased'::notification_message_status,
                status_reason = 'Leased by notifier ' || sqlc.arg('notifier_id')::int,
                leased_until = NOW() + CONCAT(sqlc.arg('lease_seconds')::int, ' seconds')::interval
            WHERE id IN (SELECT nm.id
                         FROM notification_messages AS nm
                                  LEFT JOIN notification_templates AS nt ON (nm.notification_template_id = nt.id)
                                  LEFT JOIN notification_preferences np ON (np.notification_template_id = nt.id)
                         WHERE (
                             (
                                 -- message is in acquirable states
                                 nm.status IN (
                                               'pending'::notification_message_status,
                                               'failed'::notification_message_status
                                     )
                                 )
                                 -- or somehow the message was left in leased for longer than its lease period
                                 OR (
                                 nm.status = 'leased'::notification_message_status
                                     AND nm.leased_until < NOW()
                                 )
                             )
                           -- exclude all messages which have exceeded the max attempts; these will be purged later
                           AND (nm.attempt_count IS NULL OR nm.attempt_count < sqlc.arg('max_attempt_count')::int)
                           -- if set, do not retry until we've exceeded the wait time
                           AND (
                             CASE
                                 WHEN nm.next_retry_after IS NOT NULL THEN nm.next_retry_after < NOW()
                                 ELSE true
                                 END
                             )
                           -- only lease if user/org has not disabled this template
                           -- TODO: validate this
                           AND (
                             CASE
                                 WHEN np.disabled = FALSE THEN
                                     (np.user_id = sqlc.arg('user_id')::uuid OR np.org_id = sqlc.arg('org_id')::uuid)
                                 ELSE TRUE
                                 END
                             )
                         ORDER BY nm.created_at ASC
                         -- Ensure that multiple concurrent readers cannot retrieve the same rows
                             FOR UPDATE OF nm
                                 SKIP LOCKED
                         LIMIT sqlc.arg('count'))
            RETURNING id)
SELECT
    -- message
    nm.id,
    nm.input,
    nm.targets,
    nm.method,
    -- template
    nt.name                                                    AS template_name,
    nt.title_template,
    nt.body_template,
    -- user
    nm.user_id,
    COALESCE(NULLIF(u.name, ''), NULLIF(u.username, ''))::text AS user_name,
    u.email                                                    AS user_email
FROM acquired
         JOIN notification_messages nm ON acquired.id = nm.id
         JOIN notification_templates nt ON nm.notification_template_id = nt.id
         JOIN users u ON nm.user_id = u.id;

-- name: BulkMarkNotificationMessagesFailed :execrows
WITH new_values AS (SELECT UNNEST(@ids::uuid[])                             AS id,
                           UNNEST(@failed_ats::timestamptz[])               AS failed_at,
                           UNNEST(@statuses::notification_message_status[]) AS status,
                           UNNEST(@status_reasons::text[])                  AS status_reason)
UPDATE notification_messages
SET updated_at       = subquery.failed_at,
    attempt_count    = attempt_count + 1,
    status           = subquery.status,
    status_reason    = subquery.status_reason,
    leased_until     = NULL,
    next_retry_after = CASE
                           WHEN (attempt_count + 1 < @max_attempts::int)
                               THEN NOW() + CONCAT(@retry_interval::int, ' seconds')::interval END
FROM (SELECT id, status, status_reason, failed_at
      FROM new_values) AS subquery
WHERE notification_messages.id = subquery.id;

-- name: BulkMarkNotificationMessagesInhibited :execrows
UPDATE notification_messages
SET updated_at       = NOW(),
    status           = 'inhibited'::notification_message_status,
    status_reason    = sqlc.narg('reason'),
    next_retry_after = NULL
WHERE notification_messages.id IN (UNNEST(@ids::uuid[]));

-- name: BulkMarkNotificationMessagesSent :execrows
WITH new_values AS (SELECT UNNEST(@ids::uuid[])             AS id,
                           UNNEST(@sent_ats::timestamptz[]) AS sent_at)
UPDATE notification_messages
SET updated_at       = subquery.sent_at,
    attempt_count    = attempt_count + 1,
    status           = 'sent'::notification_message_status,
    status_reason    = NULL,
    leased_until     = NULL,
    next_retry_after = NULL
FROM (SELECT id, sent_at
      FROM new_values) AS subquery
WHERE notification_messages.id = subquery.id;

-- Delete all notification messages which have not been updated for over a week.
-- name: DeleteOldNotificationMessages :exec
DELETE
FROM notification_messages
WHERE id =
      (SELECT id
       FROM notification_messages AS nested
       WHERE nested.updated_at < NOW() - INTERVAL '7 days'
          -- ensure we don't clash with the notifier
           FOR UPDATE SKIP LOCKED);

-- name: GetNotificationMessagesCountByStatus :many
SELECT status, COUNT(*) as "count"
FROM notification_messages
GROUP BY status;

-- name: GetNotificationsMessagesCountByTemplate :many
SELECT nt.id, COUNT(*) as "count"
FROM notification_messages nm
         INNER JOIN notification_templates nt ON (nm.notification_template_id = nt.id)
GROUP BY nt.id;