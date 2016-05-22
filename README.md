# systemd-monitor

systemd-monitor tails the journal log for unit failures and sends out email notifications.

## Install
```zsh
go get github.com/nhooyr/systemd-monitor
```

## Usage
```
[$] systemd-monitor --help
Usage of systemd-monitor:
  -c string
	   path to configuration file (default "/usr/local/etc/systemd-monitor/config.toml")
```

## Example
```toml
[[accounts]]
username = "user@domain.com"
password = "durp"
addr = "host:port"
[[accounts.destinations]]
     name = "Hoora Foo"
     email = "user@domain.com"
[accounts.backup]
     username = "user@gmail.com"
     password = "bar"
     addr = "smtp.gmail.com:587"
     [[accounts.backup.destinations]]
          name = "Ding Doodle"
          email = "user@domain.com"
```

Its very self explanatory but if you have any questions, feel free to email me.
