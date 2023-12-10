Steps for running the app
- clone the git repository
- building the app `go build`
- copy .env.example to .env `cp .env.example .env`
- update the .env file
- run the app

Usage:
  ```dspend [command]```

Available Commands:
  create-tx   Create a new transaction
  help        Help about any command
  modify-tx   Modify an existing transaction
  send-tx     Send a signed transaction
  view-tx     View transaction

Flags:
  -h, --help   help for dspend

Use "dspend [command] --help" for more information about a command.