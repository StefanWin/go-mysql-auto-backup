# go-mysql-auto-backup

## Currently not tested! Use at your own risk!

Backup a MySQL/MariaDB database and a data directory (e.g. user uploads etc.)  
in intervals and archive the last `n-1` backups after `n` backups.

# Requirements
- `mysqldump`
- `zip`
- `rsync`

e.g. on Ubuntu `sudo apt install mysqldump zip rsync`

# Configuration
See `config.json` for reference.

- `log_file_path` Log file
- `db.db_name` Database name
- `db.db_user` Database user
- `db.db_pw` Database password
- `data_path` Path to the data directory
- `backups_path` Path to the target directory for the backups (will be created if it does not exists)
- `archive_paths` Path to the target director for zip archives (will be created if it does not exists)
- `every_x_days` Day interval for backups
- `archive_after_x` Number of backups before archiving the last `n-1` backups

# Usage
- `go-mysql-auto-backup -config="path/to/config.json"`
- `go run main.go -config="path/to/config.json"`

Run either of those commands as `systemd` unit.