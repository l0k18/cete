package cete

import (
	"errors"
	"os"
	"sync"

	"github.com/dgraph-io/badger/badger"
	"gopkg.in/vmihailenco/msgpack.v2"
)

type indexConfig struct {
	IndexName string
}

type tableConfig struct {
	TableName string
	Indexes   []indexConfig
}

type dbConfig struct {
	Tables []tableConfig
}

func (db *DB) newKV(names ...Name) (*badger.KV, error) {
	dir := db.path

	for _, name := range names {
		dir += "/" + name.Hex()
	}

	dir += "/data"

	if err := os.MkdirAll(dir, 0744); err != nil {
		return nil, err
	}

	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	return badger.NewKV(&opts)
}

// OpenDatabase opens the database at the provided path. It will create a new
// database if the folder does not exist.
func OpenDatabase(path string) (*DB, error) {
	db := &DB{
		path:        path,
		tables:      make(map[Name]*Table),
		configMutex: new(sync.Mutex),
	}

	if ex, _ := exists(path); !ex {
		if err := os.MkdirAll(path, 0744); err != nil {
			return nil, errors.New("cete: failed to create database: " +
				err.Error())
		}

		return db, nil
	}

	file, err := os.Open(path + "/config.dat")
	if err != nil {
		return nil, errors.New("cete: failed to open database configuration. " +
			"If this is new database, please delete the database folder first: " +
			err.Error())
	}

	dec := msgpack.NewDecoder(file)
	var config dbConfig
	err = dec.Decode(&config)
	if err != nil {
		return nil, errors.New("cete: failed to read database configuration: " +
			err.Error())
	}

	db.config = config

	for _, table := range config.Tables {
		tb := &Table{indexes: make(map[Name]*Index)}
		for _, index := range table.Indexes {
			idx := &Index{}

			idx.index, err = db.newKV(Name(table.TableName), Name(index.IndexName))
			if err != nil {
				return nil, errors.New("cete: failed to open " +
					table.TableName + "/" +
					index.IndexName + ": " + err.Error())
			}
			idx.table = tb

			tb.indexes[Name(index.IndexName)] = idx
		}

		tb.data, err = db.newKV(Name(table.TableName))
		if err != nil {
			return nil, errors.New("cete: failed to open " +
				table.TableName + ": " + err.Error())
		}
		tb.db = db

		db.tables[Name(table.TableName)] = tb
	}

	return db, nil
}

func (db *DB) writeConfig() error {
	file, err := os.Create(db.path + "/config.dat")
	if err != nil {
		return err
	}

	return msgpack.NewEncoder(file).Encode(db.config)
}
