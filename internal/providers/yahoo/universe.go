package yahoo

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
)

// Universe retrieves the complete matching screener result, never a fixed ticker list.
// Policy and ranking are applied separately to the provider's real attributes.
func (c *Client) Universe(ctx context.Context) ([]models.Company, error) {
	out := []models.Company{}
	seen := map[string]bool{}
	for offset := 0; offset < 10000; offset += 250 {
		query := map[string]any{"operator": "AND", "operands": []any{
			map[string]any{"operator": "eq", "operands": []any{"region", "us"}},
			map[string]any{"operator": "gte", "operands": []any{"intradaymarketcap", 1e9}},
		}}
		body := map[string]any{"size": 250, "offset": offset, "sortField": "intradaymarketcap", "sortType": "DESC", "quoteType": "EQUITY", "query": query, "userId": "", "userIdType": "guid"}
		var r struct {
			Finance struct {
				Error  any
				Result []struct {
					Total  int
					Quotes []struct {
						Symbol, LongName, ShortName, Exchange, QuoteType, Currency, Sector, Industry string
						MarketCap, RegularMarketPrice                                                *float64
					}
				}
			}
		}
		if err := c.request(ctx, "POST", "/v1/finance/screener?lang=en-US&region=US", body, &r); err != nil {
			return nil, err
		}
		if r.Finance.Error != nil || len(r.Finance.Result) != 1 {
			return nil, providers.ErrMalformed
		}
		v := r.Finance.Result[0]
		for _, q := range v.Quotes {
			sym, err := models.Symbol(q.Symbol)
			if err != nil || seen[sym] {
				continue
			}
			seen[sym] = true
			name := q.LongName
			if name == "" {
				name = q.ShortName
			}
			out = append(out, models.Company{Symbol: sym, Name: models.Text(name), Exchange: models.Text(q.Exchange), ExchangeCode: models.Text(q.Exchange), SecurityType: models.Text(q.QuoteType), Currency: models.Text(q.Currency), MarketCap: valid(q.MarketCap), LatestPrice: positive(q.RegularMarketPrice), Sector: models.Text(q.Sector), Industry: models.Text(q.Industry)})
		}
		if offset+len(v.Quotes) >= v.Total {
			return out, nil
		}
		if len(v.Quotes) == 0 {
			return nil, providers.ErrMalformed
		}
	}
	return nil, providers.ErrMalformed
}
