package yahoo

import (
	"earnings-dashboard/internal/models"
	"encoding/json"
	"testing"
	"time"
)

func TestYahooFiscalLabelUsesHistoricalFiscalQuarter(t *testing.T) {
	var h fiscalHistory
	if err := json.Unmarshal([]byte(`{"quoteSummary":{"result":[{"earnings":{"earningsChart":{"quarterly":[{"fiscalQuarter":"1Q2026","calendarQuarter":"2Q2025","periodEndDate":{"raw":1745971200},"reportedDate":{"raw":1748390400}}],"currentFiscalQuarter":"2Q2026"}}}]}}`), &h); err != nil {
		t.Fatal(err)
	}
	rows := []models.Financial{{PeriodEnd: time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC)}}
	applyFiscalHistory(rows, h, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if rows[0].FiscalYear == nil || *rows[0].FiscalYear != 2026 || *rows[0].FiscalQuarter != 1 {
		t.Fatal(rows)
	}
	rows[0].FiscalYear = nil
	rows[0].FiscalQuarter = nil
	applyFiscalHistory(rows, h, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if rows[0].FiscalYear != nil {
		t.Fatal("forward period labeled as historical")
	}
}
