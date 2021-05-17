// MIT License
// Copyright (c) 2021 Stefan Wintergerst
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Config struct {
	DB struct {
		Name     string `json:"name"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"db"`
	LogPath     string `json:"log_file_path"`
	DataPath    string `json:"data_path"`
	BackupPath  string `json:"backups_path"`
	ArchivePath string `json:"archive_path"`
	DayInterval int    `json:"every_x_days"`
	Threshhold  int    `json:"archive_after_x"`
}

// directoryExists checks whether the directory is exists
func directoryExists(directory string) bool {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return false
	}
	return true
}

// ensureDir creates a directory if it does not exist.
func ensureDir(directory string) error {
	if !directoryExists(directory) {
		err := os.Mkdir(directory, 0777)
		if err != nil {
			return fmt.Errorf("failed to create directory : %s", directory)
		}
	}
	log.Printf("created directory: %s\n", directory)
	return nil
}

// checkRequirements checks if the requirements are in $PATH.
func checkRequirements() error {
	_, err := exec.LookPath("mysqldump")
	if err != nil {
		return err
	}
	_, err = exec.LookPath("rsync")
	return err
}

// setCmdOut sets the command output to the logger output.
func setCmdOut(cmd *exec.Cmd) {
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
}

// rsyncData copies the src to the destination.
func rsyncData(src, dst string) error {
	cmd := exec.Command("rsync", "-a", src, dst)
	setCmdOut(cmd)
	log.Printf("running command : '%s'\n", cmd.String())
	return cmd.Run()
}

// mysqldump exports the database to the destination via mysqldump.
func mysqldump(user, pw, db, dst string) error {
	cmd := exec.Command("mysqldump", "-u", user, fmt.Sprintf("-p%s", pw), db)
	dumpFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer dumpFile.Close()
	cmd.Stdout = dumpFile
	// setCmdOut(cmd)
	log.Printf("running command : '%s'\n", cmd.String())
	return cmd.Run()
}

func main() {
	// Parse command line flags
	var configPath string
	flag.StringVar(&configPath, "config", "config.json", "Path to the JSON configuration file.")
	flag.Parse()
	// Read config file
	cfgD, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("error reading config: %v", err)
	}
	// Parse config file
	cfg := Config{}
	if err := json.Unmarshal(cfgD, &cfg); err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
	// Configure logging
	logFile, err := os.OpenFile(cfg.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	if err := checkRequirements(); err != nil {
		log.Fatalf("did not find requirements: %v", err)
	}

	if !directoryExists(cfg.DataPath) {
		log.Fatal("did not find data directory")
	}

	// Create necessary directories
	if err := ensureDir(cfg.BackupPath); err != nil {
		log.Fatalf("error creating backup directory: %v", err)
	}
	if err := ensureDir(cfg.ArchivePath); err != nil {
		log.Fatalf("error creating archive directory: %v", err)
	}

	count := 0
	backupStamps := make([]string, 0)
	for {
		// create time format
		timestamp := time.Now()
		dir := timestamp.Format("2006-01-02")
		log.Println("##############################################################")
		log.Printf("running backup #%d : %s\n", count, dir)

		// create subdirectory within backup directory
		backupPath := filepath.Join(cfg.BackupPath, dir)
		if err := ensureDir(backupPath); err != nil {
			log.Fatalf("failed to create backup directory: %v", err)
		}

		// export database to current backup directory
		dumpFileName := fmt.Sprintf("%s-%s.sql", cfg.DB.Name, dir)
		dumpFilePath := filepath.Join(backupPath, dumpFileName)
		if err := mysqldump(cfg.DB.User, cfg.DB.Password, cfg.DB.Name, dumpFilePath); err != nil {
			log.Fatalf("failed to create sql dump: %v", err)
		}

		// copy data to current backup directory
		dataBackupPath := backupPath
		if err := rsyncData(cfg.DataPath, dataBackupPath); err != nil {
			log.Fatalf("failed to run rsync: %v", err)
		}

		// store subdirectory
		backupStamps = append(backupStamps, backupPath)
		count++
		if count == cfg.Threshhold {
			log.Printf("%d backups: starting cleanup\n", count)
			// remove the last n-1 backups
			if err := rsyncData(backupPath, cfg.ArchivePath); err != nil {
				log.Fatalf("failed to move directory: %s -> %s\n", backupPath, cfg.ArchivePath)
			}
			for _, dir := range backupStamps[:cfg.Threshhold-1] {
				if directoryExists(dir) {
					log.Printf("removing directory: %s\n", dir)
					if err := os.RemoveAll(dir); err != nil {
						log.Fatalf("error while removing directory: %v", err)
					}
				}
			}
			count = 0
			backupStamps = make([]string, 0)
		}
		log.Println("##############################################################")
		// sleep sweet summer child
		time.Sleep(time.Hour * 24 * time.Duration(cfg.DayInterval))
	}
}
