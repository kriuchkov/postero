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

func FormatAddresses(addresses []*mail.Address) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if rendered := strings.TrimSpace(AddressForHumans(address)); rendered != "" {
			formatted = append(formatted, rendered)
		}
	}
	return strings.Join(formatted, ", ")
}
