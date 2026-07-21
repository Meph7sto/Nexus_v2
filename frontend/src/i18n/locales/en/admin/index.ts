import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import quotaSummary from './quotaSummary'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...quotaSummary,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...promptAudit,
}
