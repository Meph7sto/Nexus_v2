export default {
  openAIQuotaSummary: {
    title: 'OpenAI / Codex 配额汇总',
    description: '按账号分组和套餐类型查看配额余量。',
    allGroups: '全部分组',
    ungrouped: '未分组',
    allTypes: '全部套餐类型',
    unknownPlan: '未知',
    current: '当前',
    hoursLater: '若干小时后',
    daysLater: '若干天后',
    projectionAmount: '预测数量',
    projection: '预测时间',
    generated: '生成时间',
    rows: '{count} 种套餐类型',
    noPermission: '你没有查看此配额汇总的权限。',
    loadFailed: '加载配额汇总失败。',
    partialSnapshot: '部分快照',
    table: {
      type: '套餐类型',
      included: '计入',
      errors: '错误',
      inactive: '未启用',
      other: '其他排除',
      missing5h: '缺少 5h',
      missing7d: '缺少 7d',
      avg5h: '5h 余量',
      avg7d: '7d 余量',
      next5hRecovery: '下次 5h 恢复',
      next7dRecovery: '下次 7d 恢复'
    }
  }
}
