// Package report renders a HappyX report as a PNG image for the guild admin.
package report

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/moixblau/HappyX.Bot/internal/api"
)

// Palette: light chart surface, text inks and a single series hue (one measure per chart,
// so no legend is needed and colour carries no identity).
var (
	colSurface   = hex("#fcfcfb")
	colTile      = hex("#f0efec")
	colText      = hex("#0b0b0b")
	colText2     = hex("#52514e")
	colGrid      = hex("#e4e3df")
	colMuted     = hex("#9a9893") // labels of days without answers
	colSeries    = hex("#2a78d6")
	colSeriesDim = hex("#cde2fb")
)

// MoodLabels are the 1-5 answers, worst first.
var MoodLabels = []string{"Muy mal", "Mal", "Normal", "Bien", "Genial"}

const (
	width   = 1200
	pad     = 56.0
	dateFmt = "2006-01-02"
)

var weekdays = []string{"do", "lu", "ma", "mi", "ju", "vi", "sá"}

var (
	regular = mustFont(goregular.TTF)
	medium  = mustFont(gomedium.TTF)
	bold    = mustFont(gobold.TTF)
)

// Render draws the report. guildName is shown in the title.
func Render(guildName string, r api.Report) ([]byte, error) {
	height := 1080
	if r.Responses == 0 {
		height = 480
	}
	dc := gg.NewContext(width, height)
	dc.SetColor(colSurface)
	dc.Clear()

	// Header
	setFont(dc, bold, 40)
	dc.SetColor(colText)
	dc.DrawString(truncate(dc, "HappyX · "+guildName, width-2*pad), pad, 88)
	setFont(dc, regular, 24)
	dc.SetColor(colText2)
	dc.DrawString(periodLabel(r.From, r.To), pad, 128)

	// Headline tiles
	avg, resp, people := "—", "0", "0"
	if r.Responses > 0 {
		avg = fmt.Sprintf("%.1f", r.Average)
		resp = fmt.Sprint(r.Responses)
		people = fmt.Sprint(r.Respondents)
	}
	tiles := []struct{ label, value, unit string }{
		{"Ánimo medio", avg, "/ 5"},
		{"Respuestas", resp, ""},
		{"Personas", people, ""},
	}
	tileGap := 24.0
	tileW := (width - 2*pad - 2*tileGap) / 3
	for i, t := range tiles {
		x := pad + float64(i)*(tileW+tileGap)
		drawTile(dc, x, 164, tileW, 140, t.label, t.value, t.unit)
	}

	if r.Responses == 0 {
		setFont(dc, medium, 26)
		dc.SetColor(colText2)
		dc.DrawStringAnchored("No hubo respuestas en este periodo.", width/2, 380, 0.5, 0.5)
		drawFooter(dc, r, float64(height))
		return encode(dc)
	}

	drawDailyChart(dc, r.Days, 360, 330)
	drawDistribution(dc, r, 740)
	drawFooter(dc, r, float64(height))
	return encode(dc)
}

func drawTile(dc *gg.Context, x, y, w, h float64, label, value, unit string) {
	dc.SetColor(colTile)
	dc.DrawRoundedRectangle(x, y, w, h, 16)
	dc.Fill()
	setFont(dc, regular, 22)
	dc.SetColor(colText2)
	dc.DrawString(label, x+24, y+42)
	setFont(dc, bold, 56)
	dc.SetColor(colText)
	dc.DrawString(value, x+24, y+112)
	if unit != "" {
		vw, _ := dc.MeasureString(value)
		setFont(dc, regular, 26)
		dc.SetColor(colText2)
		dc.DrawString(unit, x+24+vw+10, y+112)
	}
}

// drawDailyChart plots the daily average mood as a line on the 1-5 scale. Days without
// answers break the line.
func drawDailyChart(dc *gg.Context, days []api.ReportDay, top, h float64) {
	setFont(dc, medium, 26)
	dc.SetColor(colText)
	dc.DrawString("Ánimo medio por día", pad, top)

	labelW := 120.0
	left, right := pad+labelW, float64(width)-pad
	plotTop, plotBottom := top+40, top+h-50
	yOf := func(v float64) float64 { return plotBottom - (v-1)/4*(plotBottom-plotTop) }

	// Grid and mood labels (recessive)
	setFont(dc, regular, 20)
	for m := 1; m <= 5; m++ {
		y := yOf(float64(m))
		dc.SetColor(colGrid)
		dc.SetLineWidth(1)
		dc.DrawLine(left, y, right, y)
		dc.Stroke()
		dc.SetColor(colText2)
		dc.DrawStringAnchored(MoodLabels[m-1], left-16, y, 1, 0.35)
	}
	if len(days) == 0 {
		return
	}

	step := 0.0
	if len(days) > 1 {
		step = (right - left - 40) / float64(len(days)-1)
	}
	xOf := func(i int) float64 {
		if len(days) == 1 {
			return (left + right) / 2
		}
		return left + 20 + float64(i)*step
	}

	// X labels: every day for a week, about every fifth day for a month.
	every := 1
	if len(days) > 10 {
		every = 5
	}
	setFont(dc, regular, 20)
	for i, d := range days {
		if i%every != 0 && i != len(days)-1 {
			continue
		}
		dc.SetColor(colText2)
		if d.Responses == 0 {
			dc.SetColor(colMuted)
		}
		dc.DrawStringAnchored(dayLabel(d.Day, len(days) <= 10), xOf(i), plotBottom+32, 0.5, 0)
	}

	// Line segments between consecutive answered days.
	dc.SetColor(colSeries)
	dc.SetLineWidth(3)
	dc.SetLineCap(gg.LineCapRound)
	dc.SetLineJoin(gg.LineJoinRound)
	drawing := false
	for i, d := range days {
		if d.Responses == 0 {
			if drawing {
				dc.Stroke()
			}
			drawing = false
			continue
		}
		if !drawing {
			dc.MoveTo(xOf(i), yOf(d.Average))
			drawing = true
		} else {
			dc.LineTo(xOf(i), yOf(d.Average))
		}
	}
	if drawing {
		dc.Stroke()
	}

	// Markers with a surface ring so they read over the line. Days without answers get no
	// marker (a dot at the bottom would read as "Muy mal"); their x label is muted instead.
	for i, d := range days {
		x := xOf(i)
		if d.Responses == 0 {
			continue
		}
		y := yOf(d.Average)
		dc.SetColor(colSurface)
		dc.DrawCircle(x, y, 8)
		dc.Fill()
		dc.SetColor(colSeries)
		dc.DrawCircle(x, y, 6)
		dc.Fill()
	}
}

// drawDistribution shows how many answers each mood got, best first.
func drawDistribution(dc *gg.Context, r api.Report, top float64) {
	setFont(dc, medium, 26)
	dc.SetColor(colText)
	dc.DrawString("Distribución de respuestas", pad, top)

	labelW := 120.0
	valueW := 130.0
	left := pad + labelW
	maxW := float64(width) - pad - valueW - left
	rowH, gap := 34.0, 14.0
	maxN := 0
	for m := 1; m <= 5; m++ {
		maxN = max(maxN, r.Distribution[fmt.Sprint(m)])
	}
	for row, m := 0, 5; m >= 1; row, m = row+1, m-1 {
		n := r.Distribution[fmt.Sprint(m)]
		y := top + 32 + float64(row)*(rowH+gap)
		setFont(dc, regular, 20)
		dc.SetColor(colText2)
		dc.DrawStringAnchored(MoodLabels[m-1], left-16, y+rowH/2, 1, 0.35)

		// Track, then the bar: square at the baseline, rounded at the data end.
		dc.SetColor(colSeriesDim)
		dc.DrawRoundedRectangle(left, y, maxW, rowH, 4)
		dc.Fill()
		w := 0.0
		if maxN > 0 {
			w = math.Round(float64(n) / float64(maxN) * maxW)
		}
		if w > 0 {
			dc.SetColor(colSeries)
			dc.DrawRoundedRectangle(left, y, w, rowH, 4)
			dc.Fill()
			dc.DrawRectangle(left, y, math.Min(4, w), rowH)
			dc.Fill()
		}

		setFont(dc, medium, 20)
		dc.SetColor(colText)
		pct := 0
		if r.Responses > 0 {
			pct = int(math.Round(float64(n) * 100 / float64(r.Responses)))
		}
		dc.DrawStringAnchored(fmt.Sprintf("%d  (%d%%)", n, pct), left+maxW+16, y+rowH/2, 0, 0.35)
	}
}

func drawFooter(dc *gg.Context, r api.Report, height float64) {
	text := "Respuestas anónimas"
	if !r.Anonymous {
		text = "Respuestas con nombre"
	}
	if len(r.Messages) > 0 {
		if len(r.Messages) == 1 {
			text += " · 1 comentario en el mensaje"
		} else {
			text += fmt.Sprintf(" · %d comentarios en el mensaje", len(r.Messages))
		}
	}
	setFont(dc, regular, 20)
	dc.SetColor(colText2)
	dc.DrawString(text, pad, height-32)
}

// --- helpers ---

func periodLabel(from, to string) string {
	f, err1 := time.Parse(dateFmt, from)
	t, err2 := time.Parse(dateFmt, to)
	if err1 != nil || err2 != nil {
		return from + " – " + to
	}
	months := []string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	return fmt.Sprintf("Del %d %s al %d %s de %d", f.Day(), months[f.Month()-1], t.Day(), months[t.Month()-1], t.Year())
}

func dayLabel(day string, withWeekday bool) string {
	d, err := time.Parse(dateFmt, day)
	if err != nil {
		return day
	}
	if withWeekday {
		return fmt.Sprintf("%s %d", weekdays[d.Weekday()], d.Day())
	}
	return fmt.Sprint(d.Day())
}

func truncate(dc *gg.Context, s string, maxW float64) string {
	if w, _ := dc.MeasureString(s); w <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if w, _ := dc.MeasureString(string(r) + "…"); w <= maxW {
			return string(r) + "…"
		}
	}
	return ""
}

func encode(dc *gg.Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mustFont(ttf []byte) *truetype.Font {
	f, err := truetype.Parse(ttf)
	if err != nil {
		panic(err)
	}
	return f
}

func setFont(dc *gg.Context, f *truetype.Font, size float64) {
	dc.SetFontFace(truetype.NewFace(f, &truetype.Options{Size: size, Hinting: font.HintingFull}))
}

func hex(s string) color.Color {
	var r, g, b uint8
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{r, g, b, 255}
}
