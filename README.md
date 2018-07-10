# RRL2epub

Convert RoyalRoadL & Webnovel fictions into Epub 3 files.

## Usage

### Go

1. Download/Copy this repo
2. install Go dependencies & build/compile app
3. Using a terminal go to the directory of the binary and then use the following command:

./RRL2epub [url]

`[url]` May be a full path to the fiction's page, or it may be a shortened url with a specified schema. for example:

    ./RRL2epub rrl:1001
    ./RRL2epub wn:100000000000001

In this example, the "url" beginning with `rrl:` will find the fiction with the given ID number on RoyalRoadL.com, while `wn:` is meant for WebNovel.com.

If you have the binary in your PATH, go to the directory you wish to download the EPUB to, and use the same command without the `./` at the start.

### Docker (if you don't have Go installed locally)

1. Download/Copy this repo
2. Open the folder in terminal/powershell/etc & run `docker-compose run -v ${pwd}/out:/out rrl2epub ./app wn:11119470606261105`
3. The epub will appear in the generated `./out` folder