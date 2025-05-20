# Home Assistant Add-on: Duplicati

## About

If you want to create more precise backups of your Home Assistant than what the built-in tools offer, [Duplicati](https://duplicati.com/) is a helpful solution. It allows you to back up either the entire system or selected files locally or to a wide range of cloud storage providers.

It is a free, open-source backup client that securely stores encrypted, incremental, and compressed backups on cloud storage services and remote file servers. It supports:

Amazon S3, IDrive e2, Backblaze (B2), Box, Dropbox, FTP, Google Cloud and Drive, MEGA, Microsoft Azure and OneDrive, Rackspace Cloud Files, OpenStack Storage (Swift), Sia, Storj DCS, SSH (SFTP), WebDAV, Tencent Cloud Object Storage (COS), Aliyun OSS, and more!

## Installation

1. Add my add-ons repository to your home assistant instance (in supervisor add-ons store at top right, or click button below if you have configured my HA)<br>
   [![Add repository to my Home Assistant][repository-badge]][repository-url] 
2. Add the add-on to your Home Assistant by clicking on the following button:<br>
   [![Add add-on to my Home Assistant][addon-add-badge]][addon-add-url]
3. Click on **Install**
4. Ensure the checkboxes **Start on boot** and **Show in sidebar** are checked.
5. Click on **Start**
6. Open Duplicati from the sidebar usiing the **Duplicati** entry or use the following button:<br>
   [![Open add-on on my Home Assistant][addon-open-badge]][addon-open-url]
7. Configure your first job. See [Shares](#shares)

## Shares
All shares of the Home Assistant are located at `/homeassistant`.

## More
* [All echocat's Home Assistant Add-ons](https://github.com/echocat/hassos-addons)

[repository-badge]: https://img.shields.io/badge/Add%20repository%20to%20my-Home%20Assistant-41BDF5?logo=home-assistant&style=for-the-badge
[repository-url]: https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fechocat%2Fhassos-addons
[addon-add-badge]: https://img.shields.io/badge/Add%20add--on%20to%20my-Home%20Assistant-41BDF5?logo=home-assistant&style=for-the-badge
[addon-add-url]: https://my.home-assistant.io/redirect/supervisor_addon/?addon=62dd30da_duplicati&repository_url=https%3A%2F%2Fgithub.com%2Fechocat%2Fhassos-addons
[addon-open-badge]: https://img.shields.io/badge/Open%20add--on%20on%20my-Home%20Assistant-41BDF5?logo=home-assistant&style=for-the-badge
[addon-open-url]: https://my.home-assistant.io/redirect/supervisor_ingress/?addon=62dd30da_duplicati
