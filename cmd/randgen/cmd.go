package main

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"go-randgen/gendata"
	"go-randgen/grammar"
	"go-randgen/resource"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var format bool
var breake bool
var zzPath string
var yyPath string
var outPath string
var rootCmd = &cobra.Command{
	Use:   "go port for randgen",
	Short: "random generate sql with yy and zz like mysql randgen",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if yyPath == "" {
			return errors.New("yy are required")
		}
		return nil
	},
	Run: randgenAction,
}

// init command flag
func init() {
	rootCmd.Flags().BoolVarP(&format, "format", "F", true,
		"generate sql that is convenient for reading")
	rootCmd.Flags().BoolVarP(&breake, "break", "B", false,
		"break zz yy result to two resource")
	rootCmd.Flags().StringVarP(&zzPath, "zz", "Z","", "zz file path, go randgen have a default zz")
	rootCmd.Flags().StringVarP(&yyPath, "yy", "Y","", "yy file path, required")
	rootCmd.Flags().StringVarP(&outPath, "output", "o","output", "sql output file path")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(rootCmd.UsageString())
		os.Exit(1)
	}
}

// generate all sqls and write them into file
func randgenAction(cmd *cobra.Command, args []string) {
	var zzBs []byte
	var err error
	if zzPath == "" {
		log.Println("load default zz")
		zzBs, err = resource.Asset("resource/default.zz.lua")
	} else {
		zzBs, err = ioutil.ReadFile(zzPath)
	}

	if err != nil {
		log.Fatalf("load zz fail, %v\n", err)
	}

	zz := string(zzBs)

	yyBs, err := ioutil.ReadFile(yyPath)
	if err != nil {
		log.Fatalf("load yy from %s fail, %v\n", yyPath, err)
	}

	yy := string(yyBs)

	ddls, err := gendata.ByZz(zz)
	if err != nil {
		log.Fatalln(err)
	}

	randomSqls, err := grammar.ByYy(yy)
	if err != nil {
		log.Fatalln(err)
	}

	if breake {
		err := ioutil.WriteFile(outPath+".data.sql",
			[]byte(strings.Join(ddls, "\n")), os.ModePerm)
		if err != nil {
			log.Printf("write ddl in dist fail, %v\n", err)
		}

		err = ioutil.WriteFile(outPath+".rand.sql",
			[]byte(strings.Join(randomSqls, "\n")), os.ModePerm)
		if err != nil {
			log.Printf("write random sql in dist fail, %v\n", err)
		}
	} else {
		allSqls := make([]string, 0)
		allSqls = append(allSqls, ddls...)
		allSqls = append(allSqls, randomSqls...)

		err = ioutil.WriteFile(outPath + ".sql",
			[]byte(strings.Join(allSqls, "\n")), os.ModePerm)
		if err != nil {
			log.Printf("sql output error, %v\n", err)
		}
	}
}
