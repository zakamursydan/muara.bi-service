package server

import "muara.bi/core/internal/repository/pgsql"

type Repository struct {
	UserRepo pgsql.UserRepositoryProvider
}

func NewRepository(internalConnection InternalConnection) Repository {
	return Repository{
		UserRepo: pgsql.NewUserRepository(internalConnection.DB),
	}
}
