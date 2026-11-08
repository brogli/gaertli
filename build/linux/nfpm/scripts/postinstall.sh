#!/bin/sh
# Based on the Wails v3 build template (github.com/wailsapp/wails, MIT).

# This makes the application appear in application menus and registers its capabilities.
if command -v update-desktop-database >/dev/null 2>&1; then
  echo "Updating desktop database..."
  update-desktop-database -q /usr/share/applications
else
  echo "Warning: update-desktop-database command not found. Desktop file may not be immediately recognized." >&2
fi

if command -v update-mime-database >/dev/null 2>&1; then
  echo "Updating MIME database..."
  update-mime-database -n /usr/share/mime
else
  echo "Warning: update-mime-database command not found. Custom URL schemes may not be immediately recognized." >&2
fi

# Ubuntu's AppArmor denies user namespaces to programs without a profile; the
# web view's sandbox needs them. The deb installs the profile; this loads it
# into a running AppArmor.
if [ -f /etc/apparmor.d/gaertli ] && [ -d /sys/kernel/security/apparmor ] && command -v apparmor_parser >/dev/null 2>&1; then
  apparmor_parser -r -W -T /etc/apparmor.d/gaertli || echo "Warning: could not load the AppArmor profile; the app's web view may fail to start its sandbox." >&2
fi

exit 0
