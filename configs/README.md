# Config files
Other editors might work but I recommended using vscode / codium where `mirrors.json` file will automatically be checked against the `mirrors.schema.json`. Some projects may require authentication, in that case all files that match `*.secret` will not be tracked.

For example `blender.secret` will be a rsync password file with just the user's password. Make sure the secret files is not system readable.

```
touch blender.secret
chmod 600 blender.secret
$EDITOR blender.secret
```

## `tokens.txt`

This is where secret tokens are stored that allow projects to be manually synced by visiting a special url with these tokens in the query string.

format:
```
random text that doesn't match is ignored

blender:someLongSecret
```