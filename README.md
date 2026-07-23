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

That's it. If you want to delete stuff (you don't NEED to...), just `ssh` and delete it from there. 
This is totally for minimalism and not because I was too lazy to code it in.

Potato Share is blazingly fast, and on a decently modern CPU as the server,
can load a folder with thousands of files in less than a second.

No license too. It needs to be dead simple. Feel free to do whatever the heck you want with it.
PLEASE, PLEASE do not put this on somewhere accessible outside your local network. That would allow anyone
to upload random junk to your server since there is no authorization (again, DEAD simple). This is meant for
more of a homelab thing. I might consider making it a bit more professional in the future.

## Build

Although there are prebuilt binaries, please run the following:
```bash
go build
```

And you're done.

## Running

`./potato-share [SHARE_DIR] [PORT]`
`SHARE_DIR`: Directory to share to, without the / at the end, e.g. "/home/ebayan/vscode"
`PORT`: Port to host on, will host at 0.0.0.0:PORT. You can feel free to port forward this.


The default password is `potato`. Change this in the admin panel.
