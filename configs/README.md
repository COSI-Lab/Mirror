# Config files

Other editors might work but I recommended using vscode / codium where `mirrors.json` file will automatically be checked against the `mirrors.schema.json`. Some projects may require authentication, in that case all files that match `*.secret` will not be tracked.

For example `blender.secret` will be a rsync password file with just the user's password. Make sure the secret files is not system readable.

```text
touch blender.secret
chmod 600 blender.secret
$EDITOR blender.secret
```

## Third party configs

Some projects ask that syncs be preformed using a separate script. Typically these scripts are rsync wrappers. Currently these configs are for third party scripts.

| project | config                                               | script                                                       |
| :------ | :--------------------------------------------------- | :----------------------------------------------------------- |  
| debian  | [ftpsync.conf](ftpsync.conf)                         | [archvsync](https://github.com/COSI-Lab/archvsync)(forked)   |
| fedora  | [quick-fedora-mirror.conf](quick-fedora-mirror.conf) | [quick-fedora-mirror](https://pagure.io/quick-fedora-mirror) |

## `tokens.txt`

This is where secret tokens are stored that allow projects to be manually synced by visiting a special url with these tokens in the query string.

format:

```text
random text that doesn't match is ignored

blender:someLongSecret
```
