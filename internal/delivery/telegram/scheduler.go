package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/model"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strconv"
	"strings"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleScheduler(ctx context.Context, c telebot.Context) error {

	jobs, err := t.service.SchedulerService.GetJobSchedule(ctx, model.GetJobParam{
		IsActive: utils.ToPointer(true),
	})
	if err != nil {
		t.log.ErrorContext(ctx, "failed to get jobs", logger.ErrorField(err))
		_, err = t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if len(jobs) == 0 {
		_, err = t.telegram.Send(ctx, c, "Tidak ada job yang aktif.")
		return err
	}

	msg := strings.Builder{}
	msg.WriteString("📋 Daftar Scheduler Aktif:\n\n")

	msg.WriteString("<i>👉 Tekan tombol di bawah untuk lihat detail dan jalankan manual</i>\n")

	menu := &telebot.ReplyMarkup{}
	rows := []telebot.Row{}

	for _, job := range jobs {
		btn := menu.Data(job.Name, btnDetailJob.Unique, fmt.Sprintf("%d", job.ID))
		rows = append(rows, menu.Row(btn))
	}
	rows = append(rows, menu.Row(menu.Data(btnDeleteMessage.Text, btnDeleteMessage.Unique)))
	menu.Inline(rows...)

	msgExist := c.Message()
	if msgExist != nil && msgExist.Sender.ID == t.bot.Me.ID {
		_, err = t.telegram.Edit(ctx, c, msgExist, msg.String(), menu, telebot.ModeHTML)
		return err
	}

	_, err = t.telegram.Send(ctx, c, msg.String(), menu, telebot.ModeHTML)
	return err
}

func (t *TelegramBotHandler) handleBtnDetailJob(ctx context.Context, c telebot.Context) error {
	jobID := c.Data()
	jobIDInt, err := strconv.Atoi(jobID)
	if err != nil {
		t.log.ErrorContext(ctx, "failed to convert job id to int", logger.ErrorField(err))
		_, err = t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	jobs, err := t.service.SchedulerService.GetJobSchedule(ctx, model.GetJobParam{
		IDs: []uint{uint(jobIDInt)},
		WithTaskHistory: &model.GetTaskExecutionHistoryParam{
			Limit: utils.ToPointer(5),
		},
	})
	if err != nil {
		t.log.ErrorContext(ctx, "failed to get job by id", logger.ErrorField(err))
		_, err = t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if len(jobs) == 0 {
		_, err = t.telegram.Send(ctx, c, "Job tidak ditemukan.")
		return err
	}

	job := jobs[0]

	msg := strings.Builder{}
	msg.WriteString(fmt.Sprintf("%s\n\n", job.Name))
	msg.WriteString(fmt.Sprintf("🔍 %s\n\n", job.Description))

	msg.WriteString("📅 Jadwal: \n")
	if job.Schedules[0].LastExecution.Valid {
		msg.WriteString(fmt.Sprintf(" • Last Execution : %s\n", utils.PrettyDate(utils.TimeToWIB(job.Schedules[0].LastExecution.Time))))
	} else {
		msg.WriteString(" • Last Execution : Tidak ada\n")
	}
	if job.Schedules[0].NextExecution.Valid {
		msg.WriteString(fmt.Sprintf(" • Next Execution : %s\n", utils.PrettyDate(utils.TimeToWIB(job.Schedules[0].NextExecution.Time))))
	} else {
		msg.WriteString(" • Next Execution : Tidak ada\n")
	}

	msg.WriteString("\n")
	msg.WriteString("📜 Riwayat Eksekusi Terakhir:\n")
	for idx, history := range job.Histories {

		icon := "🟢"
		if history.Status == model.StatusRunning {
			icon = "🟡"
		} else if history.Status == model.StatusCompleted {
			icon = "🟢"
		} else if history.Status == model.StatusFailed {
			icon = "🔴"
		} else if history.Status == model.StatusTimeout {
			icon = "🟠"
		}

		if history.CreatedAt.IsZero() {
			continue
		}
		if !history.CompletedAt.Valid {
			msg.WriteString(fmt.Sprintf("%d. %s %s - %s\n", idx+1, icon, utils.TimeToWIB(history.CreatedAt).Format("01/02 15:04"), strings.ToUpper(string(history.Status))))
			continue
		}
		duration := history.CompletedAt.Time.Sub(history.StartedAt)
		msg.WriteString(fmt.Sprintf("%d. %s %s - %d | %s (%.1fs)\n", idx+1, icon, utils.TimeToWIB(history.CreatedAt).Format("01/02 15:04"), history.ExitCode.Int32, strings.ToUpper(string(history.Status)), duration.Seconds()))
	}

	menu := &telebot.ReplyMarkup{}
	rows := []telebot.Row{}
	btnBackJobList := menu.Data(btnActionBackToJobList.Text, btnActionBackToJobList.Unique)
	btnRun := menu.Data(btnActionRunJob.Text, btnActionRunJob.Unique, fmt.Sprintf("%d", job.ID))
	rows = append(rows, menu.Row(btnRun, btnBackJobList))
	menu.Inline(rows...)

	_, err = t.telegram.Edit(ctx, c, c.Message(), msg.String(), menu, telebot.ModeHTML)
	return err
}

func (t *TelegramBotHandler) handleBtnActionRunJob(ctx context.Context, c telebot.Context) error {
	jobID := c.Data()
	jobIDInt, err := strconv.Atoi(jobID)
	if err != nil {
		t.log.ErrorContext(ctx, "failed to convert job id to int", logger.ErrorField(err))
		_, err = t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	job, err := t.service.SchedulerService.GetJobSchedule(ctx, model.GetJobParam{
		IDs: []uint{uint(jobIDInt)},
	})
	if err != nil {
		t.log.ErrorContext(ctx, "failed to get job by id", logger.ErrorField(err))
		_, err = t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if len(job) == 0 {
		_, err = t.telegram.Send(ctx, c, "Job tidak ditemukan.")
		return err
	}

	if err := t.service.SchedulerService.RunJobTask(ctx, uint(job[0].ID)); err != nil {
		t.log.ErrorContext(ctx, "failed to run job task", logger.ErrorField(err))
		_, err = t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}
	return t.handleBtnActionBackToJobList(ctx, c)
}

func (t *TelegramBotHandler) handleBtnActionBackToJobList(ctx context.Context, c telebot.Context) error {
	return t.handleScheduler(ctx, c)
}
