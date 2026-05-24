package main

import (
	"errors"

	"gameclustering.com/internal/protocol"
	"github.com/jackc/pgx/v5"
)

const (
	CREATE_LOGIN_SCHEMA    string = "CREATE TABLE IF NOT EXISTS login (id SERIAL PRIMARY KEY,name VARCHAR(100) NOT NULL UNIQUE,hash VARCHAR(255) NOT NULL,reference_id INTEGER DEFAULT 0,access_control INTEGER DEFAULT 1)"
	INSERT_LOGIN           string = "INSERT INTO login (name,hash,access_control) VALUES($1,$2,$3)"
	SELECT_LOGIN_WITH_NAME string = "SELECT hash,id,access_control FROM login WHERE name=$1"
	UPDATE_HASH            string = "UPDATE login SET hash = $1 WHERE name = $2"
)

func (s *AdminService) createSchema() error {
	_, err := s.Sql.Exec(CREATE_LOGIN_SCHEMA)
	if err != nil {
		return err
	}
	return nil
}

func (s *AdminService) SaveLogin(login *protocol.LoginObject) error {
	inserted, err := s.Sql.Exec(INSERT_LOGIN, login.Name, login.Password, login.AccessControl)
	if err != nil {
		return err
	}
	if inserted == 0 {
		return errors.New("login cannot be saved")
	}
	return nil
}

func (s *AdminService) LoadLogin(login *protocol.LoginObject) error {
	err := s.Sql.Query(func(rows pgx.Rows) error {
		var hash string
		var id int32
		var accessControl int32
		err := rows.Scan(&hash, &id, &accessControl)
		if err != nil {
			return err
		}
		login.Password = hash
		login.Id = uint32(id)
		login.AccessControl = uint32(accessControl)
		return nil
	}, SELECT_LOGIN_WITH_NAME, login.Name)
	if err != nil {
		return err
	}
	if login.Id == 0 {
		return errors.New("login not existed")
	}
	return nil
}

func (s *AdminService) UpdatePassword(login *protocol.LoginObject) error {
	updated, err := s.Sql.Exec(UPDATE_HASH, login.Password, login.Name)
	if err != nil {
		return err
	}
	if updated == 0 {
		return errors.New("password cannot be saved")
	}
	return nil
}
