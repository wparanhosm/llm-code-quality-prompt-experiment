package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"p2p-transfer/internal/domain"
	"p2p-transfer/internal/usecase"
	"p2p-transfer/internal/usecase/mocks"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mustWallet(t *testing.T, id, owner string, balance domain.Money, status domain.WalletStatus) *domain.Wallet {
	t.Helper()
	w, err := domain.NewWallet(id, owner, balance, status)
	if err != nil {
		t.Fatalf("falha ao construir wallet de teste: %v", err)
	}
	return w
}

func TestNewTransferUseCase_PanicsOnNilDependencies(t *testing.T) {
	t.Parallel()

	t.Run("uow nil", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if recover() == nil {
				t.Fatal("esperava panic com UnitOfWork nil")
			}
		}()
		usecase.NewTransferUseCase(nil, &mocks.StaticIDGenerator{ID: "x"}, testLogger())
	})

	t.Run("ids nil", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if recover() == nil {
				t.Fatal("esperava panic com IDGenerator nil")
			}
		}()
		uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(), mocks.NewFakeTransactionRepository())
		usecase.NewTransferUseCase(uow, nil, testLogger())
	})

	t.Run("logger nil não deve causar panic (usa default)", func(t *testing.T) {
		t.Parallel()
		uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(), mocks.NewFakeTransactionRepository())
		uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "x"}, nil)
		if uc == nil {
			t.Fatal("esperava usecase não nil")
		}
	})
}

func TestTransferUseCase_Execute_Success(t *testing.T) {
	t.Parallel()

	from := mustWallet(t, "wallet-a", "owner-a", 10000, domain.WalletStatusActive)
	to := mustWallet(t, "wallet-b", "owner-b", 500, domain.WalletStatusActive)

	walletRepo := mocks.NewFakeWalletRepository(from, to)
	txRepo := mocks.NewFakeTransactionRepository()
	uow := mocks.NewFakeUnitOfWork(walletRepo, txRepo)
	ids := &mocks.StaticIDGenerator{ID: "tx-123"}

	uc := usecase.NewTransferUseCase(uow, ids, testLogger())

	out, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  3000,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if out.TransactionID != "tx-123" {
		t.Errorf("TransactionID = %q, want %q", out.TransactionID, "tx-123")
	}
	if out.FromNewBalance != 7000 {
		t.Errorf("FromNewBalance = %d, want 7000", out.FromNewBalance)
	}
	if out.ToNewBalance != 3500 {
		t.Errorf("ToNewBalance = %d, want 3500", out.ToNewBalance)
	}

	updatedFrom, _ := walletRepo.Get("wallet-a")
	if updatedFrom.Balance != 7000 {
		t.Errorf("saldo persistido da origem = %d, want 7000", updatedFrom.Balance)
	}
	updatedTo, _ := walletRepo.Get("wallet-b")
	if updatedTo.Balance != 3500 {
		t.Errorf("saldo persistido do destino = %d, want 3500", updatedTo.Balance)
	}

	if walletRepo.SavedCount() != 2 {
		t.Errorf("SavedCount = %d, want 2", walletRepo.SavedCount())
	}

	saved := txRepo.Saved()
	if len(saved) != 1 {
		t.Fatalf("esperava 1 transação salva, obteve %d", len(saved))
	}
	if saved[0].Status != domain.TransactionStatusCompleted {
		t.Errorf("status da transação salva = %s, want COMPLETED", saved[0].Status)
	}
}

func TestTransferUseCase_Execute_InvalidAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		amount int64
	}{
		{"valor zero", 0},
		{"valor negativo", -100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
			to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
			uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(from, to), mocks.NewFakeTransactionRepository())
			uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

			_, err := uc.Execute(context.Background(), usecase.TransferInput{
				FromWalletID: "wallet-a",
				ToWalletID:   "wallet-b",
				AmountCents:  tt.amount,
			})
			if !errors.Is(err, domain.ErrInvalidAmount) {
				t.Fatalf("esperava ErrInvalidAmount, obteve %v", err)
			}
		})
	}
}

func TestTransferUseCase_Execute_SameWallet(t *testing.T) {
	t.Parallel()

	from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
	uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(from), mocks.NewFakeTransactionRepository())
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-a",
		AmountCents:  100,
	})
	if !errors.Is(err, domain.ErrSameWallet) {
		t.Fatalf("esperava ErrSameWallet, obteve %v", err)
	}
}

func TestTransferUseCase_Execute_EmptyWalletIDs(t *testing.T) {
	t.Parallel()

	uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(), mocks.NewFakeTransactionRepository())
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "",
		ToWalletID:   "wallet-b",
		AmountCents:  100,
	})
	if err == nil {
		t.Fatal("esperava erro para from wallet id vazio")
	}

	_, err = uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "",
		AmountCents:  100,
	})
	if err == nil {
		t.Fatal("esperava erro para to wallet id vazio")
	}
}

func TestTransferUseCase_Execute_InsufficientFunds(t *testing.T) {
	t.Parallel()

	from := mustWallet(t, "wallet-a", "owner-a", 100, domain.WalletStatusActive)
	to := mustWallet(t, "wallet-b", "owner-b", 100, domain.WalletStatusActive)
	walletRepo := mocks.NewFakeWalletRepository(from, to)
	uow := mocks.NewFakeUnitOfWork(walletRepo, mocks.NewFakeTransactionRepository())
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  200,
	})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("esperava ErrInsufficientFunds, obteve %v", err)
	}

	// Saldos não devem ter sido persistidos com alteração.
	if walletRepo.SavedCount() != 0 {
		t.Errorf("nenhuma carteira deveria ter sido salva em caso de falha, SavedCount = %d", walletRepo.SavedCount())
	}
}

func TestTransferUseCase_Execute_WalletNotFound(t *testing.T) {
	t.Parallel()

	t.Run("origem não encontrada", func(t *testing.T) {
		t.Parallel()
		to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
		uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(to), mocks.NewFakeTransactionRepository())
		uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

		_, err := uc.Execute(context.Background(), usecase.TransferInput{
			FromWalletID: "wallet-a",
			ToWalletID:   "wallet-b",
			AmountCents:  100,
		})
		if !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obteve %v", err)
		}
	})

	t.Run("destino não encontrado", func(t *testing.T) {
		t.Parallel()
		from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
		uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(from), mocks.NewFakeTransactionRepository())
		uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

		_, err := uc.Execute(context.Background(), usecase.TransferInput{
			FromWalletID: "wallet-a",
			ToWalletID:   "wallet-b",
			AmountCents:  100,
		})
		if !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obteve %v", err)
		}
	})
}

func TestTransferUseCase_Execute_InactiveWallet(t *testing.T) {
	t.Parallel()

	t.Run("origem inativa", func(t *testing.T) {
		t.Parallel()
		from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusInactive)
		to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
		uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(from, to), mocks.NewFakeTransactionRepository())
		uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

		_, err := uc.Execute(context.Background(), usecase.TransferInput{
			FromWalletID: "wallet-a",
			ToWalletID:   "wallet-b",
			AmountCents:  100,
		})
		if !errors.Is(err, domain.ErrWalletInactive) {
			t.Fatalf("esperava ErrWalletInactive, obteve %v", err)
		}
	})

	t.Run("destino inativo", func(t *testing.T) {
		t.Parallel()
		from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
		to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusInactive)
		uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(from, to), mocks.NewFakeTransactionRepository())
		uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

		_, err := uc.Execute(context.Background(), usecase.TransferInput{
			FromWalletID: "wallet-a",
			ToWalletID:   "wallet-b",
			AmountCents:  100,
		})
		if !errors.Is(err, domain.ErrWalletInactive) {
			t.Fatalf("esperava ErrWalletInactive, obteve %v", err)
		}
	})
}

// partialLockWalletRepository é um double mínimo que viola deliberadamente
// o contrato documentado de WalletRepository.LockForUpdate (retornar um
// mapa incompleto sem erro), usado exclusivamente para exercitar a
// verificação defensiva do usecase contra implementações de infraestrutura
// não conformes.
type partialLockWalletRepository struct {
	wallets map[string]*domain.Wallet
}

func (r *partialLockWalletRepository) LockForUpdate(_ context.Context, ids []string) (map[string]*domain.Wallet, error) {
	result := make(map[string]*domain.Wallet)
	for _, id := range ids {
		if w, ok := r.wallets[id]; ok {
			result[id] = w
		}
	}
	return result, nil
}

func (r *partialLockWalletRepository) Save(_ context.Context, wallet *domain.Wallet) error {
	r.wallets[wallet.ID] = wallet
	return nil
}

type onlyWalletUnitOfWork struct {
	wallets      usecase.WalletRepository
	transactions usecase.TransactionRepository
}

func (u *onlyWalletUnitOfWork) Execute(ctx context.Context, fn func(ctx context.Context, repos usecase.TransactionalRepositories) error) error {
	return fn(ctx, u)
}

func (u *onlyWalletUnitOfWork) WalletRepository() usecase.WalletRepository { return u.wallets }
func (u *onlyWalletUnitOfWork) TransactionRepository() usecase.TransactionRepository {
	return u.transactions
}

func TestTransferUseCase_Execute_DefensiveCheck_MissingWalletInLockResult(t *testing.T) {
	t.Parallel()

	t.Run("origem ausente no resultado do lock", func(t *testing.T) {
		t.Parallel()
		to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
		repo := &partialLockWalletRepository{wallets: map[string]*domain.Wallet{"wallet-b": to}}
		uow := &onlyWalletUnitOfWork{wallets: repo, transactions: mocks.NewFakeTransactionRepository()}
		uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

		_, err := uc.Execute(context.Background(), usecase.TransferInput{
			FromWalletID: "wallet-a",
			ToWalletID:   "wallet-b",
			AmountCents:  100,
		})
		if !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obteve %v", err)
		}
	})

	t.Run("destino ausente no resultado do lock", func(t *testing.T) {
		t.Parallel()
		from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
		repo := &partialLockWalletRepository{wallets: map[string]*domain.Wallet{"wallet-a": from}}
		uow := &onlyWalletUnitOfWork{wallets: repo, transactions: mocks.NewFakeTransactionRepository()}
		uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

		_, err := uc.Execute(context.Background(), usecase.TransferInput{
			FromWalletID: "wallet-a",
			ToWalletID:   "wallet-b",
			AmountCents:  100,
		})
		if !errors.Is(err, domain.ErrWalletNotFound) {
			t.Fatalf("esperava ErrWalletNotFound, obteve %v", err)
		}
	})
}

func TestTransferUseCase_Execute_DefensiveCheck_EmptyGeneratedID(t *testing.T) {
	t.Parallel()

	// Simula uma implementação de IDGenerator defeituosa que retorna um ID
	// vazio. O usecase deve propagar o erro de validação do domínio em vez
	// de persistir uma transação com identificador inválido.
	from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
	to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
	uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(from, to), mocks.NewFakeTransactionRepository())
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: ""}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  100,
	})
	if err == nil {
		t.Fatal("esperava erro para ID de transação vazio gerado por IDGenerator defeituoso")
	}
}

func TestTransferUseCase_Execute_LockForUpdateError(t *testing.T) {
	t.Parallel()

	walletRepo := mocks.NewFakeWalletRepository()
	walletRepo.LockForUpdateErr = errors.New("erro de conexão com o banco")
	uow := mocks.NewFakeUnitOfWork(walletRepo, mocks.NewFakeTransactionRepository())
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  100,
	})
	if err == nil {
		t.Fatal("esperava erro de LockForUpdate propagado")
	}
}

func TestTransferUseCase_Execute_SaveWalletError(t *testing.T) {
	t.Parallel()

	from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
	to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
	walletRepo := mocks.NewFakeWalletRepository(from, to)
	walletRepo.SaveErr = errors.New("falha ao salvar carteira")
	uow := mocks.NewFakeUnitOfWork(walletRepo, mocks.NewFakeTransactionRepository())
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  100,
	})
	if err == nil {
		t.Fatal("esperava erro de Save propagado")
	}
}

func TestTransferUseCase_Execute_SaveDestinationWalletError(t *testing.T) {
	t.Parallel()

	from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
	to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
	walletRepo := mocks.NewFakeWalletRepository(from, to)
	walletRepo.SaveErr = errors.New("falha ao salvar carteira de destino")
	walletRepo.SaveErrForID = "wallet-b"
	uow := mocks.NewFakeUnitOfWork(walletRepo, mocks.NewFakeTransactionRepository())
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  100,
	})
	if err == nil {
		t.Fatal("esperava erro de Save propagado para a carteira de destino")
	}
}

func TestTransferUseCase_Execute_SaveTransactionError(t *testing.T) {
	t.Parallel()

	from := mustWallet(t, "wallet-a", "owner-a", 1000, domain.WalletStatusActive)
	to := mustWallet(t, "wallet-b", "owner-b", 1000, domain.WalletStatusActive)
	walletRepo := mocks.NewFakeWalletRepository(from, to)
	txRepo := mocks.NewFakeTransactionRepository()
	txRepo.SaveErr = errors.New("falha ao salvar histórico")
	uow := mocks.NewFakeUnitOfWork(walletRepo, txRepo)
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  100,
	})
	if err == nil {
		t.Fatal("esperava erro de Save do histórico propagado")
	}
}

func TestTransferUseCase_Execute_UnitOfWorkExecuteError(t *testing.T) {
	t.Parallel()

	uow := mocks.NewFakeUnitOfWork(mocks.NewFakeWalletRepository(), mocks.NewFakeTransactionRepository())
	uow.ExecuteErr = errors.New("não foi possível abrir transação")
	uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-1"}, testLogger())

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "wallet-a",
		ToWalletID:   "wallet-b",
		AmountCents:  100,
	})
	if !errors.Is(err, uow.ExecuteErr) {
		t.Fatalf("esperava erro de execução da transação propagado, obteve %v", err)
	}
}

// TestTransferUseCase_Execute_ConcurrentTransfers valida que múltiplas
// transferências concorrentes sobre o repositório fake (protegido por
// mutex interno) não corrompem o estado final, simulando o comportamento
// esperado do bloqueio pessimista em produção.
func TestTransferUseCase_Execute_ConcurrentTransfers(t *testing.T) {
	t.Parallel()

	from := mustWallet(t, "wallet-a", "owner-a", 100000, domain.WalletStatusActive)
	to := mustWallet(t, "wallet-b", "owner-b", 0, domain.WalletStatusActive)
	walletRepo := mocks.NewFakeWalletRepository(from, to)
	txRepo := mocks.NewFakeTransactionRepository()
	uow := mocks.NewFakeUnitOfWork(walletRepo, txRepo)

	const numTransfers = 50
	const amountPerTransfer = 1000

	var wg sync.WaitGroup
	for i := 0; i < numTransfers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			uc := usecase.NewTransferUseCase(uow, &mocks.StaticIDGenerator{ID: "tx-concurrent"}, testLogger())
			_, _ = uc.Execute(context.Background(), usecase.TransferInput{
				FromWalletID: "wallet-a",
				ToWalletID:   "wallet-b",
				AmountCents:  amountPerTransfer,
			})
		}(i)
	}
	wg.Wait()

	// A soma total dos saldos deve ser preservada independentemente da
	// ordem de execução concorrente (nenhum dinheiro é criado ou perdido).
	finalFrom, _ := walletRepo.Get("wallet-a")
	finalTo, _ := walletRepo.Get("wallet-b")
	total := finalFrom.Balance + finalTo.Balance
	if total != 100000 {
		t.Fatalf("saldo total após transferências concorrentes = %d, want 100000 (dinheiro criado/perdido)", total)
	}
}
