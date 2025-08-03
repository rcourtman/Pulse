# Pulse Configuration Approach (Simplified)

Following the successful pattern of apps like Radarr, Sonarr, and Jellyfin, Pulse now uses a simplified configuration approach:

## How It Works

1. **Everything Through the UI**: All configuration is done through the web interface
2. **Auto-Secured Config**: The config file is automatically secured with 600 permissions
3. **No Manual Editing**: Users never need to touch config files or environment variables
4. **Zero Setup**: Works out of the box with secure defaults

## Key Benefits

- **User Friendly**: Just like Radarr/Sonarr - configure everything in the UI
- **More Secure**: Config file is automatically secured, no risk of misconfigured .env files
- **Simpler**: No need to understand environment variables or file permissions
- **Reliable**: Single source of truth, managed by the application

## For Advanced Users

While not recommended, advanced users can still:
- Use environment variables: `${VAR_NAME}` in config values
- Use file references: `file:///path/to/secret` in config values
- Edit the config file directly (changes are picked up on restart)

But these features are hidden by default - the UI is the primary way to configure Pulse.

## Implementation

- Config file at `/etc/pulse/pulse.yml` is automatically secured (mode 0600)
- All settings can be modified through the Settings page
- Credentials are stored in the config file, protected by file permissions
- No external dependencies or complex setup required