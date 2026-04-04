# go-video-player

## Description

A simple video player written in Go, built on top of low-level multimedia libraries such as FFmpeg (via astiav bindings) and SDL.

It is intended for learning, experimentation, and low-level media processing rather than being a full-featured production player.

## Installation

Go version 1.20+ is recommended

Follow installation instructions from:

go-astiav (for FFmpeg setup)
SDL2 official documentation

Install Libav

```
apt update
apt install -y \
  libavformat-dev \
  libavcodec-dev \
  libavutil-dev \
  libavdevice-dev \
  libavfilter-dev \
  libswscale-dev \
  libswresample-dev \
  ffmpeg \
  pkg-config
```
