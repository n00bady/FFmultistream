## About
A simple GUI application build with fyne.oi that uses FFmpeg to push a stream from OBS to
multiple platforms like youtube, twitch, kick, etc... and anything else that uses RTMP.

## Usage
### Requirements
- Have enough bandwidth to stream to multiple services at the same time.
- Have `FFmpeg` and `OBS` installed

### OBS settings
- Go to OBS Stream settings. 
- As Service select Custom.
- On the Server put `rtmp://127.0.0.1:1935/live`.
- On Stream Key put `test`.

### FFmultistream config
- Add the RTMP addresses and Keys for every service you want to push your stream to.
- Press the Start button.
- Press Start Streaming in OBS.

FFmpeg will receive your OBS stream and then push it as is to all the services you have added, might need to
wait a second before the services start receiving your stream.

### Notes
You can change the OBS origin port and key in the config file located at `~/.config/FFmultistream/config.toml`.  
In the same config file your can add manually your other platforms rtmp and keys if you want.  
If you first start streaming from OBS first and then click start in FFmultistream, FFmpeg might not receive your stream, 
its better to click start in FFmultistream first and then start streaming from OBS.

## Building
### Requirements
- go 1.24 or newer
- [fyne.io](https://docs.fyne.io/started/)

### Build
Just build it.
`go build .`

If you are using wayland it's probably better to build it with wayland support:
`go build -tags wayland .`
