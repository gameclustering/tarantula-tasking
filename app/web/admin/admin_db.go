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

	CREATE_REPO_SCHEMA string = "CREATE TABLE IF NOT EXISTS repo (id SERIAL PRIMARY KEY, type VARCHAR(20) NOT NULL, name VARCHAR(255) NOT NULL UNIQUE, tag VARCHAR(100), branch VARCHAR(100))"
	INSERT_REPO        string = "INSERT INTO repo (type, name, tag, branch) VALUES($1,$2,$3,$4) RETURNING id"
	DELETE_REPO        string = "DELETE FROM repo WHERE id=$1"
	SELECT_REPOS       string = "SELECT id, type, name, tag, branch FROM repo ORDER BY id"
)

type RepoRow struct {
	Id     int32  `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Branch string `json:"branch"`
}

func (s *AdminService) createSchema() error {
	if _, err := s.Sql.Exec(CREATE_LOGIN_SCHEMA); err != nil {
		return err
	}
	if _, err := s.Sql.Exec(CREATE_REPO_SCHEMA); err != nil {
		return err
	}
	return nil
}

func (s *AdminService) SaveRepo(repo *protocol.RepoObject) (int32, error) {
	var id int32
	err := s.Sql.Query(func(r pgx.Rows) error {
		return r.Scan(&id)
	}, INSERT_REPO, repo.Type, repo.Name, repo.Tag, repo.Branch)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, errors.New("repo cannot be saved")
	}
	return id, nil
}

func (s *AdminService) DeleteRepo(id int32) error {
	deleted, err := s.Sql.Exec(DELETE_REPO, id)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return errors.New("repo not found")
	}
	return nil
}

func (s *AdminService) ListRepos() ([]RepoRow, error) {
	var list []RepoRow
	err := s.Sql.Query(func(r pgx.Rows) error {
		var row RepoRow
		if err := r.Scan(&row.Id, &row.Type, &row.Name, &row.Tag, &row.Branch); err != nil {
			return err
		}
		list = append(list, row)
		return nil
	}, SELECT_REPOS)
	return list, err
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
