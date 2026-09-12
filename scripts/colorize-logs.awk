#!/usr/bin/awk -f
# Colorizes the OTel Collector's own tab-separated zap console log lines
# (timestamp\tlevel\tcaller\tmessage...). The Collector has no built-in
# color option, so this recolors the level field in place and passes
# every other field through unchanged.
BEGIN { FS = "\t"; OFS = "\t" }
{
	level = $2
	color = ""
	if (level == "error")      color = "\033[1;31m"
	else if (level == "warn")  color = "\033[1;33m"
	else if (level == "info")  color = "\033[1;36m"
	else if (level == "debug") color = "\033[2;37m"
	if (color != "") $2 = color level "\033[0m"
	print
}
