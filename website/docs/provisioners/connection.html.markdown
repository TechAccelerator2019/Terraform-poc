---
layout: "docs"
page_title: "Provisioner Connection Settings"
sidebar_current: "docs-provisioners-connection"
description: |-
  Managing connection defaults for SSH and WinRM using the `connection` block.
---

# Provisioner Connection Settings

Many provisioners require access to the remote resource. For example,
a provisioner may need to use SSH or WinRM to connect to the resource.

Any provisioner that takes actions on a remote resource needs details about how
to connect to it. You can provide these details in a nested `connection` block.

-> **Note:** In Terraform 0.11 and earlier, providers could set default values
for some connection settings, so that `connection` blocks could sometimes be
omitted. This feature was removed in 0.12 in order to reduce confusion and make
configurations more readable and predictable.

Connection blocks don't take a block label, and can be nested within either a
`resource` or a `provisioner`.

- A `connection` block nested directly within a `resource` affects all of
  that resource's provisioners.
- A `connection` block nested in a `provisioner` block only affects that
  provisioner, and overrides any resource-level connection settings.

One use case for providing multiple connections is to have an initial
provisioner connect as the `root` user to set up user accounts, and have
subsequent provisioners connect as a user with more limited permissions.

## Example usage

```hcl
# Copies the file as the root user using SSH
provisioner "file" {
  source      = "conf/myapp.conf"
  destination = "/etc/myapp.conf"

  connection {
    type     = "ssh"
    user     = "root"
    password = "${var.root_password}"
  }
}

# Copies the file as the Administrator user using WinRM
provisioner "file" {
  source      = "conf/myapp.conf"
  destination = "C:/App/myapp.conf"

  connection {
    type     = "winrm"
    user     = "Administrator"
    password = "${var.admin_password}"
  }
}
```

## The `self` Object

Expressions in `connection` blocks cannot refer to their parent resource by
name. To work around this, an additional `self` object is available within
`connection` blocks.

The `self` object represents the connection's parent resource, and any of that
resource's attributes can be accessed as attributes of `self`. For example, an
`aws_instance` resource's `public_ip` attribute can be referenced as
`self.public_ip` within its connection configuration.

-> **Technical note:** Resource references are restricted here because Terraform
uses those references to create dependencies. Referring to a resource by name
within its own block would create a dependency cycle.

## Argument Reference

**The following arguments are supported by all connection types:**

* `type` - The connection type that should be used. Valid types are `ssh` and `winrm`.  
           Defaults to `ssh`.

* `user` - The user that we should use for the connection.  
           Defaults to `root` when using type `ssh` and defaults to `Administrator` when using type `winrm`.

* `password` - The password we should use for the connection. In some cases this is
  specified by the provider.

* `host` - The address of the resource to connect to. This is usually specified by the provider.

* `port` - The port to connect to.  
           Defaults to `22` when using type `ssh` and defaults to `5985` when using type `winrm`.

* `timeout` - The timeout to wait for the connection to become available. Should be provided as a string like `30s` or `5m`.   
              Defaults to 5 minutes.

* `script_path` - The path used to copy scripts meant for remote execution.

**Additional arguments only supported by the `ssh` connection type:**

* `private_key` - The contents of an SSH key to use for the connection. These can
  be loaded from a file on disk using
  [the `file` function](/docs/configuration/functions/file.html). This takes
  preference over the password if provided.

* `certificate` - The contents of a signed CA Certificate. The certificate argument must be
  used in conjunction with a `private_key`. These can
  be loaded from a file on disk using the [the `file` function](/docs/configuration/functions/file.html).

* `agent` - Set to `false` to disable using `ssh-agent` to authenticate. On Windows the
  only supported SSH authentication agent is
  [Pageant](http://the.earth.li/~sgtatham/putty/0.66/htmldoc/Chapter9.html#pageant).

* `agent_identity` - The preferred identity from the ssh agent for authentication.

* `host_key` - The public key from the remote host or the signing CA, used to
  verify the connection.

**Additional arguments only supported by the `winrm` connection type:**

* `https` - Set to `true` to connect using HTTPS instead of HTTP.

* `insecure` - Set to `true` to not validate the HTTPS certificate chain.

* `use_ntlm` - Set to `true` to use NTLM authentication, rather than default (basic authentication), removing the requirement for basic authentication to be enabled within the target guest. Further reading for remote connection authentication can be found [here](https://msdn.microsoft.com/en-us/library/aa384295(v=vs.85).aspx).

* `cacert` - The CA certificate to validate against.

<a id="bastion"></a>

## Connecting through a Bastion Host with SSH

The `ssh` connection also supports the following fields to facilitate connnections via a
[bastion host](https://en.wikipedia.org/wiki/Bastion_host).

* `bastion_host` - Setting this enables the bastion Host connection. This host
  will be connected to first, and then the `host` connection will be made from there.

* `bastion_host_key` - The public key from the remote host or the signing CA,
  used to verify the host connection.

* `bastion_port` - The port to use connect to the bastion host. Defaults to the
  value of the `port` field.

* `bastion_user` - The user for the connection to the bastion host. Defaults to
  the value of the `user` field.

* `bastion_password` - The password we should use for the bastion host.
  Defaults to the value of the `password` field.

* `bastion_private_key` - The contents of an SSH key file to use for the bastion
  host. These can be loaded from a file on disk using
  [the `file` function](/docs/configuration/functions/file.html).
  Defaults to the value of the `private_key` field.

* `bastion_certificate` - The contents of a signed CA Certificate. The certificate argument
  must be used in conjunction with a `bastion_private_key`. These can be loaded from
  a file on disk using the [the `file` function](/docs/configuration/functions/file.html).