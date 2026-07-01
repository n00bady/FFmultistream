## About
A simple GUI application build with fyne.oi that uses FFmpeg to push a stream from OBS to multiple platforms like youtube, twitch, kick, etc... and anything else that uses RTMP.

## Major change
This doesn't use fyne for UI anymore, now its using go templates to create a web UI located at `http://127.0.0.1:8765/` by default.

## Usage
### Requirements
- Have enough bandwidth to stream to multiple services at the same time.
- Have `FFmpeg` and `OBS` installed

### OBS settings
- Go to OBS Stream settings. 
- As Service select Custom.
- On the Server put `rtmp://127.0.0.1:1935/live/`.
- On Stream Key put `test`. 

### FFmultistream
- When you start FFmultistream it should open `http://127.0.0.1:8765` on your default browser. The **username** and **password** should be shown in the terminal on the **first** run.
- Click the `+` button next to `DESTINATIONS` to add the RTMP addresses and Keys for every service you want to push your stream to.
- Press the Start button to start ffmpeg.
- Press Start Streaming in OBS.
- You can toggle the destinations you want to stream on/off by clicking the pause/resume button.
- When you finish streaming, first stop the stream from OBS and then click the Stop button in the web UI to properly end a stream, otherwise some services might think your stream dropped instead of ended.
- The config file can be found in `~/.config/FFmultistream/config.toml`.

FFmpeg listener will receive your OBS stream and then push it as is to all the services you have added, might need to wait a few seconds before the services start receiving your stream.

### Flags
- `-open=false` to stop it from opening the browser.
- `-uiaddr=(host:port)` to bind it to host:port of your choice.

### Notes
Make sure to start the FFmpeg first via the Start button and then start your stream from from OBS otherwhise OBS will not find the listener.  
When running from a remote location make sure you configure the host:port fom both the web UI and the ffmpeg Origin (you can even have multiple origins too by clicking the `+` button next to `ORIGINS`.


## Building
### Requirements
- go 1.24 or newer

### Build
Just build it.  
`go build .`

