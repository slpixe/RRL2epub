# RRL2epub

As the title and description suggests, this is a program to convert RoyalRoadL fictions into Epub files. Specifically, this tool converts them into EPUB 3.

**Update:** Tool has been updated to handle webnovels on webnovel.com as well.

## Usage

Use of this program is simple. Using a terminal, go to the directory of the binary and then use the following command:

    ./RRL2epub [url]

`[url]` May be a full path to the fiction's page, or it may be a shortened url with a specified schema. for example:

    ./RRL2epub rrl:1001
    ./RRL2epub wn:100000000000001

In this example, the "url" beginning with `rrl:` will find the fiction with the given ID number on RoyalRoadL.com, while `wn:` is meant for WebNovel.com.

If you have the binary in your PATH, go to the directory you wish to download the EPUB to, and use the same command without the `./` at the start.
