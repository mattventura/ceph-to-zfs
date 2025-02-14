# Host Setup (Debian-Based)

Prerequisites:
- Golang >= 1.22

```shell
# You need the header files for librados and librbd
# as root/sudo:
apt install librados-dev librbd-dev 
# Clone and build the app. Alternatively, download the zip (see below).
git clone https://github.com/mattventura/ceph-to-zfs.git
cd ceph-to-zfs
# If git did not make it executable, be sure to chmod +x it.
./build.sh
# Check that it built correctly
./ctz --help
```

Alternatively, if you do not wish to install git, you can use the github ZIP file instead:

```shell
# wget or curl the file
wget https://github.com/mattventura/ceph-to-zfs/archive/refs/heads/master.zip -O ctz.zip
unzip ctz.zip
cd ceph-to-zfs-master/
chmod +x build.sh
```

# ZFS Configuration

You will need to ensure that the user you wish to run CTZ as has adequate permissions to the parent dataset that you
plan to use for backups.

```shell
# Create dataset if needed
zfs create tank/ceph-backups
# If using a different user for CTZ, grant that user adequate permissions
# Permissions for RBD
zfs allow backupuser create,destroy,rollback,snapshot tank/ceph-backups
```

# CTZ Configuration

Copy the included `config.sample.yaml` to `config.yaml` and edit accordingly.

First, define one or more Ceph clusters to connect to in the `clusters` section. If you are already using Ceph on the
host system, it is still recommended that you create a separate Ceph user with the minimum required privilege on the
cluster, and use that for CTZ.

Then, configure your backup jobs. Each backup job copies RBD image(s) from the cluster to ZFS. 
The `cluster` property refers to the name you gave your cluster in the `clusters` section (`myCluster` in the
example). 
`zfsDestination` acts as the parent ZFS dataset for the images.
Volumes are created under that dataset, with the same name as they have in Ceph. In the example,
the `zfsDestination` is `tank/backups/vm-images`, so an image by the name of `vm-disk-1` would become
`tank/backups/vm-images/vm-disk-1`. 
If `imageIncludeRegex` is specified, only matching images are included. Otherwise, all images are included by default.
If `imageExcludeRegex` is specified, images matching that will be excluded. Exclusion has higher priority over
inclusion.

# Running

The two most common modes of operation are "oneshot" and "web".

## Oneshot

Oneshot mode executes all jobs and then exits.
```shell
./ctz -oneshot
```
Non-zero exit codes indicate that one or more jobs failed.

## Web

To use the web interface, specify the -web flag, and optionally the -webport flag.
```shell
./ctz -web -webport 8888
```
It will not do anything by default. However, it exposes a few endpoints on the specified port:

- `GET /api/alltasks` - display the status of all tasks. Will not have much info until tasks are started or at least prepped.
- `GET /api/prepall` - prep all tasks, but do not run them. Useful for seeing what images CTZ would process.
- `GET /api/startall` - start running all tasks.

After calling `prepall` or `startall`, check `alltasks` and/or the console output to monitor progress.