package sqlgen

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	progressbar "github.com/schollz/progressbar/v3"
)

func TestA(t *testing.T) {
	state := NewState()
	state.InjectTodoSQL("set @@tidb_enable_clustered_index=true")
	gen := NewGenerator(state)
	for i := 0; i < 200; i++ {
		fmt.Printf("%s;\n", gen())
	}
}

func initDB(name string, sqls []string, opts *ControlOption, enableClusterIndex bool) error {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:4000)/test")
	if err != nil {
		return err
	}
	db.Exec("drop database if exists " + name)
	db.Exec("create database " + name)
	err = db.Close()
	if err != nil {
		return err
	}
	db, err = sql.Open("mysql", "root:@tcp(127.0.0.1:4000)/"+name)
	if err != nil {
		return err
	}
	if enableClusterIndex {
		_, err = db.Exec("set @@tidb_enable_clustered_index = true")
		if err != nil {
			return err
		}
	} else {
		_, err = db.Exec("set @@tidb_enable_clustered_index = false")
		if err != nil {
			return err
		}
	}
	bar := progressbar.Default(int64(len(sqls)))
	for i, sql := range sqls {
		_, err = db.Exec(sql)
		if err != nil && i < opts.InitTableCount*2 {
			return err
		}
		bar.Add(1)
	}
	for i := 0; i < opts.InitTableCount; i++ {
		_, err = db.Exec("analyze table tbl_" + strconv.FormatInt(int64(i), 10))
		if err != nil {
			return err
		}
	}
	return nil
}

func parseRes(rows *sql.Rows) (string, error) {
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	// Result is your slice string.
	rawResult := make([][]byte, len(cols))
	result := make([]string, len(cols))
	results := make([][]string, 0)

	dest := make([]interface{}, len(cols)) // A temporary interface{} slice
	for i, _ := range rawResult {
		dest[i] = &rawResult[i] // Put pointers to each string in the interface slice
	}

	for rows.Next() {
		err = rows.Scan(dest...)
		if err != nil {
			return "", err
		}

		for i, raw := range rawResult {
			if raw == nil {
				result[i] = "NULL"
			} else {
				result[i] = string(raw)
			}
		}
		results = append(results, make([]string, len(cols)))
		copy(results[len(results)-1], result)
	}
	ret := ""
	for _, row := range results {
		ret += strings.Join(row, "\t")
		ret += "\n"
	}
	return ret, nil
}

func getQueryPerformance(sql string, conn *sql.DB) (avg, sum, p80, p90, p95, max, min float64, err error) {
	t := make([]time.Duration, 100)
	bar := progressbar.Default(100)
	for i := 0; i < 100; i++ {
		start := time.Now()
		_, err = conn.Exec(sql)
		t[i] = time.Since(start)
		if err != nil {
			return
		}
		sum += float64(t[i]) / 1e3
		max = math.Max(max, float64(t[i]) / 1e3)
		min = math.Min(min, float64(t[i]) / 1e3)
		bar.Add(1)
	}
	sort.Slice(t, func(i, j int) bool {
		return t[i] < t[j]
	})
	return sum / 100, sum, float64(t[79]) / 1e3, float64(t[89]) / 1e3, float64(t[95]) / 1e3, max, min, nil
}

func TestPerformance(t *testing.T) {
	state := NewState()
	state.enabledClustered = true
	state.InjectTodoSQL("drop table if exists tbl_0")
	state.InjectTodoSQL("drop table if exists tbl_1")
	state.InjectTodoSQL("drop table if exists tbl_2")
	state.InjectTodoSQL("drop table if exists tbl_3")
	state.InjectTodoSQL("drop table if exists tbl_4")
	state.ctrl.InitRowCount = 5000
	queryCnt := 100
	gen := NewGenerator(state)
	prepareData := make([]string, state.ctrl.InitTableCount*(state.ctrl.InitRowCount+2))
	queryData := make([]string, queryCnt)
	for i := 0; i < state.ctrl.InitTableCount*(state.ctrl.InitRowCount+2)+queryCnt; i++ {
		sql := gen()
		if i >= state.ctrl.InitTableCount*(state.ctrl.InitRowCount+2) &&
			(sql[:len("select")] != "select" ||
				sql[len(sql)-len("for update "):] == "for update ") {
			i--
			continue
		}
		if i < state.ctrl.InitTableCount*(state.ctrl.InitRowCount+2) {
			prepareData[i] = sql
		} else {
			queryData[i-state.ctrl.InitTableCount*(state.ctrl.InitRowCount+2)] = sql
		}
	}
	f, err := os.OpenFile("prepare_sql.sql", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	for _, sql := range prepareData {
		f.Write([]byte(sql + ";\n"))
	}
	f.Close()
	err = initDB("with_cluster_index", prepareData, state.ctrl, true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	err = initDB("wout_cluster_index", prepareData, state.ctrl, false)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	f, err = os.OpenFile("query_sql.sql", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	for _, sql := range queryData {
		f.Write([]byte(sql + ";\n"))
	}
	f.Close()

	connWith, err := sql.Open("mysql", "root:@tcp(127.0.0.1:4000)/with_cluster_index")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	connWout, err := sql.Open("mysql", "root:@tcp(127.0.0.1:4000)/wout_cluster_index")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	f, err = os.OpenFile("results", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	defer f.Close()
	// test performance
	for _, data := range queryData {
		fmt.Println("test query: " + data)
		f.Write([]byte("\n"+data + ";\n"))

		avg, sum, p80, p90, p95, max, min, err := getQueryPerformance(data, connWith)
		if err != nil {
			f.Write([]byte(err.Error() + "\n"))
			continue
		}
		f.Write([]byte(fmt.Sprintf("Performance with cluster index: \navg:%f, sum:%f, p80:%f, p90:%f, p95:%f, max:%f, min:%f\n", avg, sum, p80, p90, p95, max, min)))
		avg, sum, p80, p90, p95, max, min, err = getQueryPerformance(data, connWout)
		if err != nil {
			f.Write([]byte(err.Error() + "\n"))
			continue
		}
		f.Write([]byte(fmt.Sprintf("Performance wout cluster index: \navg:%f, sum:%f, p80:%f, p90:%f, p95:%f, max:%f, min:%f\n", avg, sum, p80, p90, p95, max, min)))

		rows, err := connWith.Query("explain " + data)
		if err != nil {
			f.Write([]byte(err.Error() + "\n"))
			continue
		}
		res, err := parseRes(rows)
		if err != nil {
			f.Write([]byte(err.Error() + "\n"))
			continue
		}
		f.Write([]byte("with_cluster_index_plan: \n" + res))
		rows, err = connWout.Query("explain " + data)
		if err != nil {
			f.Write([]byte(err.Error() + "\n"))
			continue
		}
		res, err = parseRes(rows)
		if err != nil {
			f.Write([]byte(err.Error() + "\n"))
			continue
		}
		f.Write([]byte("wout_cluster_index_plan: \n" + res))
	}
}
