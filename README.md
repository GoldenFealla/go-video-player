# go-video-player

## Description

A simple video player written in Go, built on top of low-level multimedia libraries such as FFmpeg (via astiav bindings) and SDL.

This project focuses on:

Understanding how video playback works internally
Handling decoding, rendering, and audio/video synchronization
Building a minimal media player from "scratch" in Go

It is intended for learning, experimentation, and low-level media processing rather than being a full-featured production player.

## Installation

### Requirements

Go (1.20+ recommended)
FFmpeg libraries
SDL2
Platforms
Linux
Windows
Install

Follow installation instructions from:

go-astiav (for FFmpeg setup)
SDL2 official documentation

Then run:

git clone https://github.com/GoldenFealla/go-video-player

cd go-video-player
go mod tidy

## Instruction

Run the player

go run main.go <video_file>

Example

go run main.go sample.mp4

Notes
This project uses FFmpeg decoding through Go bindings
Performance may vary between Linux and Windows
Designed for learning how media players work internally
