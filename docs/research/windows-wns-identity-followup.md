# Windows MDM WNS identity and agent wake

## Fleet and other implementations

Fleet's [WNS investigation](https://github.com/fleetdm/fleet/issues/43773)
reports successful classic authentication using `ms-app://<Package SID>`, a
client secret, and `scope=notify.windows.com`. WNS accepted a raw push with
HTTP 200 and `X-WNS-Status: received`. This establishes that Fleet's Store
identity worked with the classic flow. It does not establish that every Store
identity can obtain a token through that endpoint.

Fleet chose an agent-mediated wake instead of shipping WNS. Its
[replacement design](https://github.com/fleetdm/fleet/issues/46567) sends a
sync request through fleetd/orbit, which invokes an on-demand OMA-DM session.
It relaxes the Windows poll schedule only after that agent advertises the
capability and the device acknowledges the schedule change. Hosts without a
capable agent retain fast polling. Fleet later reported a separate Windows
`EnrollmentState=3` registry detection bug in this path
([issue 48760](https://github.com/fleetdm/fleet/issues/48760)). This is not an
agentless wake mechanism for this project.

The [local agent workflow](../testing/windows-agent-wake.md) confirms this
alternate wake on the headed test VM. A server-issued enrollment token lets
the installed agent poll for a pending command; the agent then runs
`deviceenroller.exe /o <GUID> /c`, and the read-only Get completes before
the next scheduled MDM poll. Therefore the workflow offers a tested agent
delivery path while Store WNS authentication remains unresolved.

Fleet's original account of Event 4603 said it blocked the session. A
[correction](https://learn.microsoft.com/en-us/answers/questions/5907951/wns-push-initiated-mdm-session-never-starts-on-win)
from its investigator says the session can proceed despite Event 4603. Treat
Event 4603 as a diagnostic observation, not proof of delivery failure; verify
push-attributable session arrival and command results.

[HCL BigFix MCM's current credential guide](https://help.hcl-software.com/bigfix/11.0/mcm/MCM/Install/c_wns_credentials.html)
also specifies the Partner Center PFN, `ms-app://<Package SID>`, and client
secret. This independently corroborates the documented credential format.

Rudy Ooms's [account of Intune on-demand sync](https://patchmypc.com/blog/inside-the-improved-on-demand-sync-for-windows-devices/)
describes WNS waking the Windows MDM policy session, with a separate wake for
the Intune Management Extension. This is useful context for the two delivery
paths, but Intune's service identity does not diagnose our third-party Store
token rejection.

## Similar identity reports

A [MDM developer report](https://learn.microsoft.com/en-us/answers/questions/5720820/trying-to-setup-wns-for-mdm-dm-client-notification)
resolved its token error after removing federated credentials from the linked
app registration and keeping the client secret. This is a single report with
an unspecified original error, so it does not explain our explicit
`login.live.com` rejection. It suggests a read-only check of the linked
registration's federated credentials before escalating.

A [Store SID mismatch thread](https://learn.microsoft.com/en-us/answers/questions/5848106/wns-push-notification-failing-microsoft-store-pack)
includes a developer who used the Partner Center linked app registration and
still could not obtain a token. Replies recommend Partner Center support for
backend provisioning. Some replies assert that the classic flow should work,
but do not account for our more specific endpoint error; they are not proof of
a fix. Azure Notification Hubs would still need valid WNS credentials.

Microsoft's [MDM push documentation](https://learn.microsoft.com/en-us/windows/client-management/push-notification-windows-mdm)
still directs MDM servers to a Store PFN, SID and secret. The observed
`login.live.com` error explicitly tells new clients to use Entra, while our
Partner Center linked registration is personal-account-only and Entra WNS
rejects `/consumers`. The public sources above do not identify a supported
migration for this Store PFN. Preserve its Store identity and request a
Microsoft answer or backend repair before changing the identifier URI.
