package server

import "muara.bi/core/internal/repository/pgsql"

type Repository struct {
	UserRepo       pgsql.UserRepositoryProvider
	ConnectionRepo pgsql.ConnectionRepositoryProvider
	SemanticRepo   pgsql.SemanticRepositoryProvider
}

func NewRepository(internalConnection InternalConnection) Repository {
	return Repository{
		UserRepo:       pgsql.NewUserRepository(internalConnection.DB),
		ConnectionRepo: pgsql.NewConnectionRepository(internalConnection.DB),
		SemanticRepo:   pgsql.NewSemanticRepository(internalConnection.DB),
	}
}
