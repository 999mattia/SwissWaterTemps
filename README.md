# SwissWaterTemps

Check the temperatures of lakes and rivers across switzerland.
## Disclaimer

The used data belongs to the BAFU (https://www.hydrodaten.admin.ch/de/seen-und-fluesse/messstationen-temperatur) and boot24(https://www.boot24.ch/chde/service/temperaturen/).
This is a non profit website, used for educational purposes.

## Garmin Watch App Installation
Required:

-   Garmin Connect IQ SDK (https://formulae.brew.sh/cask/connectiq)
-   Visual Studio Code (https://formulae.brew.sh/cask/visual-studio-code) with MonkeyC Extension
-   If you are on a Mac: Android File Transfer (https://formulae.brew.sh/cask/android-file-transfer#default)

1. Clone this repo
2. Open the project in VS Code
3. Open Command Palette and execute "Monkey C: Generate a Developer Key"
4. Save the developer key to desired location
5. Check in manifest.xml if your device is supoorted. If not, add it
7. Open Command Palette and execute "Monkey C: Build for Device"
8. Select your desired device
9. Save the output to desired folder
10. Connect your watch to your pc
11. Access the watch in the file explorer and open the GARMIN/Apps folder (on Mac use Android File Explorer)
12. Copy the .prg file from the output the the GARMIN/Apps folder
