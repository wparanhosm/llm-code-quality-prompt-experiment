package domain

import "time"

type Transfer struct {
	ID, SourceWalletID, DestinationWalletID string
	Amount                                  Money
	CreatedAt                               time.Time
}
