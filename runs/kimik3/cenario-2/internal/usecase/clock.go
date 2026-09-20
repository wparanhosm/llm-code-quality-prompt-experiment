package usecase

import "time"

// SystemClock é a implementação de Clock baseada no relógio do sistema.
type SystemClock struct{}

// NewSystemClock constrói um SystemClock.
func NewSystemClock() SystemClock { return SystemClock{} }

// Now retorna o instante atual em UTC.
func (SystemClock) Now() time.Time { return time.Now().UTC() }
