package types

type MessagePayload struct {
	Version string `json:"_version"`

	NotificationName string `json:"notification_name,omitempty"`
	CreatedBy        string `json:"created_by,omitempty"`

	UserID    string `json:"user_id,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
	UserName  string `json:"user_name,omitempty"`

	Labels Labels `json:"labels,omitempty"`
}
