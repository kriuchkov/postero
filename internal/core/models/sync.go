package models

// SyncTarget describes one remote mailbox to pull into the local store with
// already-resolved credentials, keeping the domain free of config concerns.
type SyncTarget struct {
	AccountName string
	Host        string
	Port        int
	Username    string
	Password    string
	AuthType    string
	TLS         bool
	Mailbox     string // defaults to INBOX
	Limit       int    // defaults to 50
}
