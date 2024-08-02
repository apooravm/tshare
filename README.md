# tshare-client
Share files between machines.

## Usage

App usage: `tshare-client.exe [COMMAND] [CMD_ARG] -[SUB_CMD]=[SUB_CMD_ARG]`

## Commands

- **Send** - Send a file. Point to any file. `tshare-client.exe send <path/to/file>`
- **Receive** - Receive a file. Custom receiver folder/path can be assigned by passing it next. `tshare-client.exe receive [CUSTOM_RECV_PATH]`
- **Help** - Display this helper text. `tshare-client.exe help`

## Subcommands

Attach these at the end:

- Set a custom chunk size. `-chunk=<CHUNK_SIZE>`
- Set a custom client name. `-name=<NAME>`
- Set to dev mode. `-mode=dev`
- Set progress bar type. all/single `-pbtype=single`
- Set progress bar length. Default is 20. `-pblen=50`
- Set progress bar rgb colouring. rgb/normal `-pbcolour=rgb`
- Set progress bar size unit. mb/kb `-pbunit=mb`
- Turn off the progress bar. `-pb=off`

---

Since 15-11-2023
