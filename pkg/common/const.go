package common

const (
	KEY_STOCK_PRICE_ALERT = "stock_price_alert:%s:%s"
	KEY_LAST_PRICE        = "last_price:%s"
)

const (
	CRYPTO = "CRYPTO"
)

const (
	EXCHANGE_IDX     = "IDX"
	EXCHANGE_NASDAQ  = "NASDAQ"
	EXCHANGE_BINANCE = "BINANCE"
)

func GetExchangeList() []string {
	return []string{
		EXCHANGE_IDX,
		EXCHANGE_NASDAQ,
		EXCHANGE_BINANCE,
	}
}

const (
	KEY_LOG_HOOK_SEND_ALERT = "send_alert"
)
