package postgres_test

import (
	"context"
	"errors"
	"testing"

	"p2p-transfer/internal/adapter/repository/postgres"
	"p2p-transfer/internal/usecase"
)

func TestUnitOfWork_Execute_CommitsOnSuccess(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectCommit()

	called := false
	err := uow.Execute(context.Background(), func(ctx context.Context, repos usecase.TransactionalRepositories) error {
		called = true
		if repos.WalletRepository() == nil {
			t.Fatal("WalletRepository não deveria ser nil")
		}
		if repos.TransactionRepository() == nil {
			t.Fatal("TransactionRepository não deveria ser nil")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !called {
		t.Fatal("função fn deveria ter sido chamada")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas do mock não atendidas: %v", err)
	}
}

func TestUnitOfWork_Execute_RollsBackOnError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectRollback()

	expectedErr := errors.New("falha de negócio")
	err := uow.Execute(context.Background(), func(ctx context.Context, repos usecase.TransactionalRepositories) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("esperava erro de negócio propagado, obteve %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas do mock não atendidas: %v", err)
	}
}

func TestUnitOfWork_Execute_BeginError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin().WillReturnError(errors.New("erro ao conectar"))

	err := uow.Execute(context.Background(), func(ctx context.Context, repos usecase.TransactionalRepositories) error {
		t.Fatal("fn não deveria ser chamada se Begin falhar")
		return nil
	})
	if err == nil {
		t.Fatal("esperava erro de Begin propagado")
	}
}

func TestUnitOfWork_Execute_CommitError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("falha ao commitar"))

	err := uow.Execute(context.Background(), func(ctx context.Context, repos usecase.TransactionalRepositories) error {
		return nil
	})
	if err == nil {
		t.Fatal("esperava erro de commit propagado")
	}
}

func TestUnitOfWork_Execute_RollbackErrorAfterBusinessError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errors.New("conexão perdida durante rollback"))

	businessErr := errors.New("falha de negócio")
	err := uow.Execute(context.Background(), func(ctx context.Context, repos usecase.TransactionalRepositories) error {
		return businessErr
	})
	if err == nil {
		t.Fatal("esperava erro combinado propagado")
	}
}

func TestUnitOfWork_Execute_RecoversFromPanicAndRollsBack(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectRollback()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("esperava panic relançado")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("expectativas do mock não atendidas: %v", err)
		}
	}()

	_ = uow.Execute(context.Background(), func(ctx context.Context, repos usecase.TransactionalRepositories) error {
		panic("erro catastrófico inesperado")
	})
}
