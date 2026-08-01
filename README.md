# Potato Share

Basically google drive but hostable on a potato.

## Prerequisites

* Go
* GNU/Linux
* a drive
* some ram

### Features

Potato Share is built to be as dead simple as possible. It features:
* A list view
* A grid view
* File uploading
* Making new folders
* DELETING STUFF (new)

That's it. ~~If you want to delete stuff (you don't NEED to...), just `ssh` and delete it from there. 
This is totally for minimalism and not because I was too lazy to code it in.~~

Potato Share is blazingly fast, and on a decently modern CPU as the server,
can load a folder with thousands of files in less than a second.

No license too. It needs to be dead simple. Feel free to do whatever the heck you want with it.
PLEASE, PLEASE do not put this on somewhere accessible outside your local network. Although I have tried
to make a password security system, it isn't PERFECT. This is meant for
more of a homelab thing. I might consider making it a bit more professional in the future.

## Obtaining
```bash
git clone https://github.com/potato-master369/potato-share
```

Or, download this repo as a ZIP.

## Build

Although there are prebuilt binaries, please run the following:
```bash
./compile.sh
```
This script contains a compilation command that removes debug stuff. It makes the final binary smaller (<10MB!).


If you want debug stuff for whatever reason, just run `go build` to get a normal build.


And you're done.

## Running

`./potato-share [SHARE_DIR] [PORT]`
`SHARE_DIR`: Directory to share to, without the / at the end, e.g. "/home/ebayan/vscode"
`PORT`: Port to host on, will host at 0.0.0.0:PORT. You can feel free to port forward this.


The default password is `potato`. Change this in the admin panel.

You may install Potato Share themes, by copying the theme folder into `./themes`. The directory should look like
```
themes/
    my-theme/
        index.html
        ...
```

and apply them using `./applytheme.sh [THEME NAME]`. Recently, the default theme has been set to Default+ because it lowkey looks better. It is free to use as well.
