package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/utils"
	"sort"
	"strings"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleMyPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID

	positions, err := t.service.TelegramBotService.GetStockPositions(ctx, dto.GetStockPositionsParam{
		TelegramID: &userID,
		IsActive:   utils.ToPointer(true),
		Monitoring: &dto.StockPositionMonitoringQueryParam{
			ShowNewest: utils.ToPointer(true),
		},
	})
	if err != nil {
		t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if len(positions) == 0 {
		t.telegram.Send(ctx, c, "❌ Tidak ada saham aktif yang kamu set position saat ini.")
		return nil
	}

	return t.showMyPosition(ctx, c, positions)
}

func (t *TelegramBotHandler) showMyPosition(ctx context.Context, c telebot.Context, positions []model.StockPosition) error {
	sb := strings.Builder{}
	header := `📊 Posisi Saham yang Kamu Pantau Saat ini:`
	sb.WriteString(header)
	sb.WriteString("\n\n")

	sort.Slice(positions, func(i, j int) bool {
		return positions[i].FinalScore > positions[j].FinalScore
	})

	for idx, position := range positions {
		stockWithExchange := position.Exchange + ":" + position.StockCode
		pnl := "N/A"
		marketPrice := float64(0)

		isHasMonitoring := len(position.StockPositionMonitorings) > 0
		stockCodeWithExchange := position.Exchange + ":" + position.StockCode
		marketPrice, _ = cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange))

		if marketPrice == 0 && isHasMonitoring {
			marketPrice = position.StockPositionMonitorings[0].MarketPrice
		}

		if marketPrice > 0 {
			pnl = utils.FormatChange(position.BuyPrice, marketPrice)
		}

		iconTitle := "⚪️"
		if marketPrice > position.BuyPrice {
			iconTitle = "🟢"
		} else if marketPrice < position.BuyPrice {
			iconTitle = "🔴"
		}
		sb.WriteString(fmt.Sprintf("<b>%d. %s %s</b>\n", idx+1, iconTitle, stockWithExchange))
		sb.WriteString(fmt.Sprintf("  • Buy: %s ⮕ %s (%s)\n", utils.FormatPrice(position.BuyPrice, position.Exchange), utils.FormatPrice(marketPrice, position.Exchange), pnl))
		if position.TrailingProfitPrice > 0 {
			sb.WriteString(fmt.Sprintf("  • TP: %s ⮕ %s (%s)\n", utils.FormatPrice(position.TakeProfitPrice, position.Exchange), utils.FormatPrice(position.TrailingProfitPrice, position.Exchange), utils.FormatChange(position.BuyPrice, position.TrailingProfitPrice)))
		} else {
			sb.WriteString(fmt.Sprintf("  • TP: %s (%s)\n", utils.FormatPrice(position.TakeProfitPrice, position.Exchange), utils.FormatChange(position.BuyPrice, position.TakeProfitPrice)))
		}
		if position.TrailingStopPrice > 0 {
			sb.WriteString(fmt.Sprintf("  • SL: %s ⮕ %s (%s)\n", utils.FormatPrice(position.StopLossPrice, position.Exchange), utils.FormatPrice(position.TrailingStopPrice, position.Exchange), utils.FormatChange(position.BuyPrice, position.TrailingStopPrice)))
		} else {
			sb.WriteString(fmt.Sprintf("  • SL: %s (%s)\n", utils.FormatPrice(position.StopLossPrice, position.Exchange), utils.FormatChange(position.BuyPrice, position.StopLossPrice)))
		}

		var (
			techScore  string
			techStatus string
			signal     string
		)

		if !isHasMonitoring {
			techScore = "N/A"
			techStatus = "N/A"
			signal = "N/A"
		} else {

			var evalSummary model.PositionAnalysisSummary

			err := json.Unmarshal(position.StockPositionMonitorings[0].EvaluationSummary, &evalSummary)
			if err != nil {
				continue
			}
			techScore = fmt.Sprintf("%.2f ⮕ %.2f (%s)", position.InitialScore, evalSummary.TechnicalAnalysis.Score, utils.FormatChange(position.InitialScore, evalSummary.TechnicalAnalysis.Score))
			techStatus = dto.PositionStatus(evalSummary.TechnicalAnalysis.Status).String()
			signal = fmt.Sprintf("%s", dto.Signal(evalSummary.PositionSignal).String())
		}
		sb.WriteString(fmt.Sprintf("  • Score (Pos): %s\n", techScore))
		sb.WriteString(fmt.Sprintf("  • Score (Plan): %.2f\n", position.PlanScore))
		sb.WriteString(fmt.Sprintf("  • Signal: %s\n", signal))
		sb.WriteString(fmt.Sprintf("  • Status: %s\n", techStatus))

		sb.WriteString("\n")
	}

	sb.WriteString("\n👉 Tekan tombol di bawah untuk melihat detail lengkap atau mengelola posisi.")
	menu := &telebot.ReplyMarkup{}
	rows := []telebot.Row{}
	var tempRow []telebot.Btn

	for _, position := range positions {
		btn := menu.Data(position.Exchange+":"+position.StockCode, btnToDetailStockPosition.Unique, fmt.Sprintf("%d", position.ID))
		tempRow = append(tempRow, btn)
		if len(tempRow) == 2 {
			rows = append(rows, menu.Row(tempRow...))
			tempRow = []telebot.Btn{}
		}
	}

	btnDelete := menu.Data("🗑 Hapus Pesan", btnDeleteMessage.Unique)

	if len(tempRow) > 0 {
		tempRow = append(tempRow, btnDelete)
		rows = append(rows, menu.Row(tempRow...))
	} else {
		rows = append(rows, menu.Row(btnDelete))
	}

	menu.Inline(rows...)

	msgRoot := c.Message()
	if msgRoot == nil || msgRoot.Sender == nil || !msgRoot.Sender.IsBot {
		_, err := t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
		return err
	} else {
		_, err := t.telegram.Edit(ctx, c, msgRoot, sb.String(), menu, telebot.ModeHTML)
		return err
	}
}

func (t *TelegramBotHandler) handleBtnBackStockPosition(ctx context.Context, c telebot.Context) error {
	return t.handleMyPosition(ctx, c)
}
