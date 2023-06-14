package logger

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Sqlite struct {
	lock                     sync.Mutex
	DB                       *sql.DB
	file                     string
	doInsert                 bool
	purgeQueryFuncList       []PurgeQueryFunc
	createTableQueryFuncList []CreateTableQueryFunc

	logs chan Sqlable
}

func NewSqlite(file string, createTableQueryFuncList []CreateTableQueryFunc, purgeQueryFuncList []PurgeQueryFunc) *Sqlite {
	return &Sqlite{
		file:                     file,
		createTableQueryFuncList: createTableQueryFuncList,
		purgeQueryFuncList:       purgeQueryFuncList,
		logs:                     make(chan Sqlable, 200),
	}
}

func (s *Sqlite) Init(logTTL time.Duration) error {
	fmt.Println("initializing database:", s.file)
	db, err := sql.Open("sqlite", s.file)
	if err != nil {
		return fmt.Errorf("opening database: %s", err.Error())
	}

	for _, createQuery := range s.createTableQueryFuncList {
		if _, err := db.Exec(createQuery()); err != nil {
			return fmt.Errorf("creating table: %s", err.Error())
		}
	}

	fmt.Println("database initialized, will purge every:", logTTL.String())

	if logTTL > 0 {
		go func() {
			for {
				time.Sleep(time.Minute)
				err := s.Purge(logTTL)
				if err != nil {
					panic(fmt.Errorf("purging database: %s", err.Error()))
				}
			}
		}()
	}

	go func() {
		type grrr struct {
			count           int
			cumulatedParams []any
			cumulatedFields string
		}
		queries := map[string]*grrr{}
		for {
			start := time.Now()
			log := <-s.logs
			query, fields, params := log.InsertQuery()

			g, found := queries[query]
			if !found {
				g = &grrr{}
				queries[query] = g
			}
			g.count++
			g.cumulatedFields += fields
			g.cumulatedParams = append(g.cumulatedParams, params...)

			if g.count < 100 {
				continue
			}

			g.cumulatedFields = g.cumulatedFields[0 : len(g.cumulatedFields)-1] //remove last comma
			stmt, err := db.Prepare(query + g.cumulatedFields)
			if err != nil {
				panic(fmt.Errorf("preparing statement for inserting data: %w", err))
			}
			s.lock.Lock()
			_, err = stmt.Exec(g.cumulatedParams...)
			s.lock.Unlock()
			if err != nil {
				panic(fmt.Errorf("inserting data: %s", err.Error()))
			}
			delete(queries, query)
			fmt.Println("inserted data in:", time.Since(start), len(s.logs), cap(s.logs))
		}
	}()

	s.DB = db

	return nil
}

func (s *Sqlite) Purge(ttl time.Duration) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	t := time.Now().Add(ttl * -1)
	fmt.Println("purging database older than:", t)
	for _, purgeQueryFunc := range s.purgeQueryFuncList {
		if res, err := s.DB.Exec(purgeQueryFunc(), t); err != nil {
			return err
		} else {
			c, _ := res.RowsAffected()
			fmt.Println("purged rows:", c)
		}
	}

	return nil
}

func (s *Sqlite) Log(data Sqlable) error {

	s.logs <- data

	return nil
}

func (s *Sqlite) SingleRowQuery(sql string, handleRow func(row *sql.Rows) error, params ...any) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	rows, err := s.DB.Query(sql, params...)
	if err != nil {
		return fmt.Errorf("querying last position: %s", err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := handleRow(rows)
		if err != nil {
			return fmt.Errorf("handling row: %s", err.Error())
		}
		return nil
	}

	return nil
}

func (s *Sqlite) Query(debugLogQuery bool, sql string, handleRow func(row *sql.Rows) error, params []any) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if debugLogQuery {
		fmt.Println("Running query:", sql, params)
	}

	rows, err := s.DB.Query(sql, params...)
	if err != nil {
		return fmt.Errorf("querying last position: %s", err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		err := handleRow(rows)
		if err != nil {
			return fmt.Errorf("handling row: %s", err.Error())
		}
	}

	return nil
}
