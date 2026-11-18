package format

import (
	"fmt"
	"net/mail"
	"strings"
)

const rfc5322Specials = `()<>[]:;@\\,."`

func AddressForHumans(address *mail.Address) string {
	if address == nil {
		return ""
	}

	if address.Name == "" {
		return fmt.Sprintf("<%s>", address.Address)
	}

	if strings.ContainsAny(address.Name, rfc5322Specials) {
		return fmt.Sprintf("\"%s\" <%s>", strings.ReplaceAll(address.Name, "\"", "'"), address.Address)
	}

	return fmt.Sprintf("%s <%s>", address.Name, address.Address)
}

// ExtractEmail pulls the bare email address out of a header-style value such
// as `Name <user@host>`; malformed input falls back to the trimmed original.
func ExtractEmail(raw string) string {
	raw = strings.TrimSpace(raw)
	if address, err := mail.ParseAddress(raw); err == nil {
		return address.Address
	}
	if lt := strings.Index(raw, "<"); lt >= 0 {
		if gt := strings.Index(raw[lt:], ">"); gt > 1 {
			return strings.TrimSpace(raw[lt+1 : lt+gt])
		}
	}
	return raw
}

//nolint:revive // exported helper keeps package API explicit.
func FormatAddresses(addresses []*mail.Address) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if rendered := strings.TrimSpace(AddressForHumans(address)); rendered != "" {
			formatted = append(formatted, rendered)
		}
	}
	return strings.Join(formatted, ", ")
}
