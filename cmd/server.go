package cmd

import (
	"context"
	"golang-trading/internal/delivery/http"
	"golang-trading/internal/delivery/telegram"
	"golang-trading/internal/repository"
	"golang-trading/internal/service"
	"log"
	httpNet "net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Run golang-trading",
	Run:   Start,
}

func Start(cmd *cobra.Command, args []string) {

	// Create a context that is canceled on interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	appDep, err := NewAppDependency(ctx)
	if err != nil {
		log.Fatalf("Failed to create app dependency: %v", err)
	}

	repo, err := repository.NewRepository(appDep.cfg, appDep.cache, appDep.db.DB, appDep.log)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}

	services := service.NewService(
		appDep.cfg,
		appDep.log,
		repo,
		appDep.cache,
		appDep.telegram,
	)
	httpHandler := http.NewHttpAPIHandler(ctx, appDep.echo, appDep.validator, services)

	telegramHandler := telegram.NewTelegramBotHandler(
		ctx,
		appDep.cfg,
		appDep.log,
		appDep.telegramBot,
		appDep.telegram,
		appDep.echo,
		appDep.validator,
		services,
		appDep.cache,
		repo.SystemParamRepo,
	)

	apiServer := NewHTTPServer(ctx, appDep, httpHandler)
	go func() {
		if err := apiServer.Start(); err != nil && err != httpNet.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	go func() {
		telegramHandler.Start()
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	telegramHandler.Stop()

	if err := apiServer.Stop(); err != nil {
		log.Fatalf("Failed to stop HTTP server: %v", err)
	}

	if err := appDep.Close(); err != nil {
		log.Fatalf("Failed to close app dependency: %v", err)
	}
}
