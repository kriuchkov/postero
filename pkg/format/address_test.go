package format_test

import (
	"net/mail"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kriuchkov/postero/pkg/format"
)

func TestAddressForHumansNil(t *testing.T) {
	assert.Empty(t, format.AddressForHumans(nil))
}

func TestAddressForHumansNoName(t *testing.T) {
	addr := &mail.Address{Address: "alice@example.com"}
	assert.Equal(t, "<alice@example.com>", format.AddressForHumans(addr))
}

func TestAddressForHumansWithName(t *testing.T) {
	addr := &mail.Address{Name: "Alice Smith", Address: "alice@example.com"}
	assert.Equal(t, "Alice Smith <alice@example.com>", format.AddressForHumans(addr))
}

func TestAddressForHumansNameWithSpecials(t *testing.T) {
	addr := &mail.Address{Name: `Bob "The Guy" Jones`, Address: "bob@example.com"}
	result := format.AddressForHumans(addr)
	// name contains double-quote → wrapped in quotes, inner quotes replaced with single
	assert.Equal(t, `"Bob 'The Guy' Jones" <bob@example.com>`, result)
}

func TestAddressForHumansNameWithAngleBracket(t *testing.T) {
	addr := &mail.Address{Name: "Alice <alias>", Address: "alice@example.com"}
	result := format.AddressForHumans(addr)
	assert.Contains(t, result, `"Alice <alias>"`)
}

func TestAddressForHumansNameWithComma(t *testing.T) {
	addr := &mail.Address{Name: "Smith, Alice", Address: "alice@example.com"}
	result := format.AddressForHumans(addr)
	assert.Equal(t, `"Smith, Alice" <alice@example.com>`, result)
}

func TestFormatAddressesEmpty(t *testing.T) {
	assert.Empty(t, format.FormatAddresses(nil))
	assert.Empty(t, format.FormatAddresses([]*mail.Address{}))
}

func TestFormatAddressesSingle(t *testing.T) {
	addrs := []*mail.Address{{Name: "Alice", Address: "alice@example.com"}}
	assert.Equal(t, "Alice <alice@example.com>", format.FormatAddresses(addrs))
}

func TestFormatAddressesMultiple(t *testing.T) {
	addrs := []*mail.Address{
		{Name: "Alice", Address: "alice@example.com"},
		{Address: "bob@example.com"},
		{Name: "Carol", Address: "carol@example.com"},
	}
	result := format.FormatAddresses(addrs)
	assert.Equal(t, "Alice <alice@example.com>, <bob@example.com>, Carol <carol@example.com>", result)
}

func TestFormatAddressesSkipsNilEntries(t *testing.T) {
	addrs := []*mail.Address{
		{Name: "Alice", Address: "alice@example.com"},
		nil,
		{Name: "Bob", Address: "bob@example.com"},
	}
	result := format.FormatAddresses(addrs)
	assert.Equal(t, "Alice <alice@example.com>, Bob <bob@example.com>", result)
}

func TestExtractEmail(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Alice <alice@example.com>":     "alice@example.com",
		`"Bob B." <bob@example.com>`:    "bob@example.com",
		"plain@example.com":             "plain@example.com",
		"  spaced@example.com  ":        "spaced@example.com",
		"Broken <unclosed@example.com>": "unclosed@example.com",
		"just a name":                   "just a name",
	}
	for raw, want := range cases {
		assert.Equal(t, want, format.ExtractEmail(raw), raw)
	}
}
