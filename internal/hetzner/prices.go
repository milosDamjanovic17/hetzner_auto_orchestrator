package hetzner

import (
	"context"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// Prices is Hetzner's live price list, gross (incl. VAT at VATRate). Amounts
// stay the API's decimal strings: the caller parses them exactly (big.Rat),
// never through float, because this is money.
type Prices struct {
	VATRate         string        `json:"vat_rate"`
	ImagePerGBMonth string        `json:"image_per_gb_month"`
	Servers         []ServerPrice `json:"servers"`
	PrimaryIPv4     []IPPrice     `json:"primary_ipv4"`
}

// ServerPrice is one server type in one location.
type ServerPrice struct {
	Type     string `json:"type"`
	Location string `json:"location"`
	Hourly   string `json:"hourly"`
	Monthly  string `json:"monthly"` // the cap on the hourly bill
}

type IPPrice struct {
	Location string `json:"location"`
	Monthly  string `json:"monthly"`
}

// Prices fetches the price list. Never cached: every cost check shows what
// Hetzner says at that moment (phase-2b §5.1).
func (c *Client) Prices(ctx context.Context) (Prices, error) {
	p, _, err := c.api.Pricing.Get(ctx)
	if err != nil {
		return Prices{}, classify(err, "getting prices")
	}
	out := Prices{
		VATRate:         p.VATRate,
		ImagePerGBMonth: p.Image.PerGBMonth.Gross,
		Servers:         []ServerPrice{},
		PrimaryIPv4:     []IPPrice{},
	}
	for _, st := range p.ServerTypes {
		if st.ServerType == nil {
			continue
		}
		for _, lp := range st.Pricings {
			if lp.Location == nil {
				continue
			}
			out.Servers = append(out.Servers, ServerPrice{
				Type:     st.ServerType.Name,
				Location: lp.Location.Name,
				Hourly:   lp.Hourly.Gross,
				Monthly:  lp.Monthly.Gross,
			})
		}
	}
	for _, ip := range p.PrimaryIPs {
		if ip.Type != string(hcloud.PrimaryIPTypeIPv4) {
			continue
		}
		for _, lp := range ip.Pricings {
			out.PrimaryIPv4 = append(out.PrimaryIPv4, IPPrice{Location: lp.Location, Monthly: lp.Monthly.Gross})
		}
	}
	return out, nil
}
