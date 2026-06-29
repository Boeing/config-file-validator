# Config File Validator Demo
# Run: vhs demo.tape

Output website/static/img/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 900
Set Height 500
Set Theme "Dracula"
Set Padding 20
Set Framerate 30

Type "validator /tmp/cfv-demo"
Sleep 500ms
Enter
Sleep 3s

Type "validator --schemastore /tmp/cfv-demo"
Sleep 500ms
Enter
Sleep 3s
