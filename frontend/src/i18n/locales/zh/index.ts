import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import leaderboard from './leaderboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...leaderboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
}
