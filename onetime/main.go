package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

type CompetitionRow struct {
	TenantID   int64         `db:"tenant_id"`
	ID         string        `db:"id"`
	Title      string        `db:"title"`
	FinishedAt sql.NullInt64 `db:"finished_at"`
	CreatedAt  int64         `db:"created_at"`
	UpdatedAt  int64         `db:"updated_at"`
}

type PlayerScoreRow struct {
	TenantID      int64  `db:"tenant_id"`
	ID            string `db:"id"`
	PlayerID      string `db:"player_id"`
	CompetitionID string `db:"competition_id"`
	Score         int64  `db:"score"`
	RowNum        int64  `db:"row_num"`
	CreatedAt     int64  `db:"created_at"`
	UpdatedAt     int64  `db:"updated_at"`
}

var sqliteDriverName = "sqlite3"

// テナントDBのパスを返す
func tenantDBPath(id int64) string {
	return filepath.Join("../tmp/initial_data", fmt.Sprintf("%d.db", id))
}

// テナントDBに接続する
func connectToTenantDB(id int64) (*sqlx.DB, error) {
	p := tenantDBPath(id)
	db, err := sqlx.Open(sqliteDriverName, fmt.Sprintf("file:%s?mode=rw", p))
	if err != nil {
		return nil, fmt.Errorf("failed to open tenant DB: %w", err)
	}
	return db, nil
}

func connectToTmpTenantDB(id int64) (*sqlx.DB, error) {
	p := filepath.Join("../tmp/new_initial_data", fmt.Sprintf("%d.db", id))
	db, err := sqlx.Open(sqliteDriverName, fmt.Sprintf("file:%s?mode=rw", p))
	if err != nil {
		return nil, fmt.Errorf("failed to open tenant DB: %w", err)
	}
	return db, nil
}

func createTmpTenantDB(id int64) error {
	tenantDBSchemaFilePath := "../webapp/sql/tenant/10_schema.sql"
	p := filepath.Join("../tmp/new_initial_data", fmt.Sprintf("%d.db", id))

	_, err := os.Stat(p)
	if os.IsNotExist(err) {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("sqlite3 %s < %s", p, tenantDBSchemaFilePath))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to exec sqlite3 %s < %s, out=%s: %w", p, tenantDBSchemaFilePath, string(out), err)
		}
	}
	return nil
}

func createNewData(index int, playerScoreRows []PlayerScoreRow) error {
	createTmpTenantDB(int64(index))
	tmpTenantDB, err := connectToTmpTenantDB(int64(index))
	defer tmpTenantDB.Close()
	if err != nil {
		return err
	}

	rowNum := len(playerScoreRows)

	for j := 0; j < rowNum; j += 500 {
		current := len(playerScoreRows)
		_, err = tmpTenantDB.NamedExec(
			`INSERT INTO player_score (id, tenant_id, player_id, competition_id, score, row_num, created_at, updated_at) VALUES (:id, :tenant_id, :player_id, :competition_id, :score, :row_num, :created_at, :updated_at)`,
			playerScoreRows[j:int(math.Min(float64(j+500), float64(current)))],
		)

	}

	for _, score := range playerScoreRows {
		_, err = tmpTenantDB.NamedExec(
			`INSERT INTO player_score (id, tenant_id, player_id, competition_id, score, row_num, created_at, updated_at) VALUES (:id, :tenant_id, :player_id, :competition_id, :score, :row_num, :created_at, :updated_at)`,
			score,
		)
	}
	if err != nil {
		return err
		//return fmt.Errorf("INSERT INTO ERROR: %w", err)
	}
	return nil
}

func main() {
	fmt.Println("start")

	for i := 1; i < 100; i++ {
		fmt.Printf("db.%d\n", i)
		tenantDB, err := connectToTenantDB(int64(i))
		defer tenantDB.Close()
		if err != nil {
			panic(err)
		}
		var competitionRows []CompetitionRow
		var playerScoreRows []PlayerScoreRow

		if err := tenantDB.SelectContext(
			context.Background(),
			&competitionRows,
			"SELECT * FROM competition",
		); err != nil {
			panic(err)
			//return fmt.Errorf("failed to fetch competition: %w", err)
		}

		for _, com := range competitionRows {
			if err := tenantDB.SelectContext(
				context.Background(),
				&playerScoreRows,
				"SELECT * from player_score WHERE competition_id = ? GROUP BY player_id HAVING MAX(row_num);",
				com.ID,
			); err != nil {
				panic(err)
				//return fmt.Errorf("failed to fetch player_score: %w", err)
			}

			createNewData(i, playerScoreRows)
		}
	}
}
