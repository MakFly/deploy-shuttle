import { Document, Page, StyleSheet, Text, View, renderToFile } from '@react-pdf/renderer'
import { readFile } from 'node:fs/promises'

type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info'
type Status = 'passed' | 'failed' | 'skipped' | 'unknown'

type CheckResult = {
  id: string
  title: string
  category: string
  severity: Severity
  status: Status
  summary: string
  remediation?: string
  ignored?: boolean
  ignoreReason?: string
  evidence?: Record<string, unknown>
}

type Report = {
  target: string
  profile: string[]
  configPath?: string
  score: number
  level: string
  checks: CheckResult[]
  generatedAt: string
}

const styles = StyleSheet.create({
  page: {
    padding: 36,
    fontSize: 10,
    fontFamily: 'Helvetica',
    color: '#17202a',
    backgroundColor: '#ffffff',
  },
  header: {
    marginBottom: 20,
    paddingBottom: 14,
    borderBottom: '1 solid #d9e2ec',
  },
  eyebrow: {
    color: '#486581',
    fontSize: 9,
    textTransform: 'uppercase',
    marginBottom: 6,
  },
  title: {
    fontSize: 22,
    fontWeight: 700,
    marginBottom: 8,
  },
  meta: {
    color: '#52606d',
    marginTop: 2,
  },
  scoreRow: {
    flexDirection: 'row',
    gap: 12,
    marginBottom: 18,
  },
  scoreBox: {
    width: 110,
    padding: 12,
    borderRadius: 4,
    backgroundColor: '#102a43',
    color: '#ffffff',
  },
  score: {
    fontSize: 28,
    fontWeight: 700,
  },
  scoreLabel: {
    marginTop: 4,
    fontSize: 10,
  },
  summaryTitle: {
    fontSize: 11,
    fontWeight: 700,
    marginBottom: 6,
  },
  summaryBox: {
    flex: 1,
    padding: 12,
    borderRadius: 4,
    backgroundColor: '#f0f4f8',
  },
  narrative: {
    marginBottom: 12,
    padding: 10,
    borderRadius: 4,
    backgroundColor: '#f8fafc',
    border: '1 solid #d9e2ec',
    lineHeight: 1.35,
  },
  nextAction: {
    marginBottom: 5,
    lineHeight: 1.35,
  },
  section: {
    marginTop: 14,
  },
  sectionTitle: {
    fontSize: 13,
    fontWeight: 700,
    marginBottom: 8,
  },
  check: {
    marginBottom: 8,
    padding: 9,
    borderRadius: 4,
    border: '1 solid #d9e2ec',
  },
  checkTitle: {
    fontSize: 10,
    fontWeight: 700,
  },
  checkMeta: {
    marginTop: 2,
    color: '#627d98',
    fontSize: 8,
  },
  checkSummary: {
    marginTop: 5,
    lineHeight: 1.35,
  },
  remediation: {
    marginTop: 5,
    color: '#334e68',
    lineHeight: 1.35,
  },
  ignored: {
    marginTop: 5,
    color: '#7b8794',
  },
  evidence: {
    marginTop: 5,
    color: '#627d98',
    fontSize: 8,
    lineHeight: 1.3,
  },
  footer: {
    position: 'absolute',
    bottom: 18,
    left: 36,
    right: 36,
    color: '#9fb3c8',
    fontSize: 8,
    borderTop: '1 solid #edf2f7',
    paddingTop: 8,
  },
})

const severityOrder: Severity[] = ['critical', 'high', 'medium', 'low', 'info']

function label(value: string): string {
  return value
    .split('-')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function failedChecks(report: Report, severity: Severity): CheckResult[] {
  return report.checks.filter(
    (check) => check.severity === severity && check.status === 'failed' && !check.ignored,
  )
}

function ignoredChecks(report: Report): CheckResult[] {
  return report.checks.filter((check) => check.ignored)
}

function openFindings(report: Report): CheckResult[] {
  return report.checks.filter((check) => check.status === 'failed' && !check.ignored)
}

function passedCount(report: Report): number {
  return report.checks.filter((check) => check.status === 'passed').length
}

function executiveSummary(report: Report): string {
  if (report.level === 'production-ready') {
    return 'This VPS is production-ready for the selected profile. Remaining findings should still be reviewed before client handoff.'
  }
  if (report.level === 'almost-ready') {
    return 'This VPS is close to production-ready. Fix or explicitly accept the high-priority findings before relying on it for critical workloads.'
  }
  if (report.level === 'risky') {
    return 'This VPS has meaningful production risks. Address the high and medium findings before treating the setup as client-ready.'
  }
  return 'This VPS is not production-ready. Critical issues should be fixed before deploying production or client workloads.'
}

function nextActions(checks: CheckResult[]): string[] {
  return checks
    .filter((check) => Boolean(check.remediation))
    .slice(0, 5)
    .map((check) => `${check.title}: ${check.remediation}`)
}

function compactEvidenceValue(value: unknown): string {
  if (Array.isArray(value)) return value.map((item) => String(item)).join(', ')
  return String(value)
}

function evidenceSummary(check: CheckResult): string {
  if (!check.evidence) return ''
  const keys = [
    'runtimeMode',
    'publicPorts',
    'workloads',
    'ignoredWorkloads',
    'readWriteWorkloads',
    'firewallRestricted',
    'ipRestriction',
    'denyRule',
    'basicAuth',
  ]
  return keys
    .filter((key) => Object.prototype.hasOwnProperty.call(check.evidence, key))
    .map((key) => [key, compactEvidenceValue(check.evidence?.[key])] as const)
    .filter(([, value]) => value.length > 0)
    .map(([key, value]) => `${key}=${value}`)
    .join('; ')
}

function ReportDocument({ report }: { report: Report }) {
  const failed = openFindings(report)
  const ignored = ignoredChecks(report)
  const actions = nextActions(failed)

  return (
    <Document title="DeployShuttle Production Readiness Report">
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Text style={styles.eyebrow}>DeployShuttle</Text>
          <Text style={styles.title}>Production Readiness Report</Text>
          <Text style={styles.meta}>Target: {report.target}</Text>
          <Text style={styles.meta}>Generated: {report.generatedAt}</Text>
          {report.configPath ? <Text style={styles.meta}>Config: {report.configPath}</Text> : null}
        </View>

        <View style={styles.scoreRow}>
          <View style={styles.scoreBox}>
            <Text style={styles.score}>{report.score}</Text>
            <Text style={styles.scoreLabel}>/100 · {label(report.level)}</Text>
          </View>
          <View style={styles.summaryBox}>
            <Text style={styles.summaryTitle}>Executive Summary</Text>
            <Text>Profile: {report.profile.join(', ')}</Text>
            <Text>Passed checks: {passedCount(report)}</Text>
            <Text>Open findings: {failed.length}</Text>
            <Text>Accepted risks: {ignored.length}</Text>
          </View>
        </View>

        <Text style={styles.narrative}>{executiveSummary(report)}</Text>

        {actions.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Next Actions</Text>
            {actions.map((action, index) => (
              <Text key={action} style={styles.nextAction}>
                {index + 1}. {action}
              </Text>
            ))}
          </View>
        ) : null}

        {severityOrder.map((severity) => {
          const checks = failedChecks(report, severity)
          if (checks.length === 0) return null
          return (
            <View key={severity} style={styles.section}>
              <Text style={styles.sectionTitle}>{label(severity)}</Text>
              {checks.map((check) => (
                <View key={check.id} style={styles.check}>
                  <Text style={styles.checkTitle}>{check.title}</Text>
                  <Text style={styles.checkMeta}>
                    {check.id} · {check.category}
                  </Text>
                  <Text style={styles.checkSummary}>{check.summary}</Text>
                  {check.remediation ? (
                    <Text style={styles.remediation}>Fix: {check.remediation}</Text>
                  ) : null}
                  {evidenceSummary(check) ? (
                    <Text style={styles.evidence}>Evidence: {evidenceSummary(check)}</Text>
                  ) : null}
                </View>
              ))}
            </View>
          )
        })}

        {ignored.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Ignored</Text>
            {ignored.map((check) => (
              <View key={check.id} style={styles.check}>
                <Text style={styles.checkTitle}>{check.title}</Text>
                <Text style={styles.checkMeta}>{check.id}</Text>
                <Text style={styles.ignored}>{check.ignoreReason}</Text>
                {evidenceSummary(check) ? (
                  <Text style={styles.evidence}>Evidence: {evidenceSummary(check)}</Text>
                ) : null}
              </View>
            ))}
          </View>
        ) : null}

        <Text style={styles.footer}>DeployShuttle · CLI-first VPS production readiness</Text>
      </Page>
    </Document>
  )
}

function arg(name: string): string | undefined {
  const index = process.argv.indexOf(name)
  if (index === -1) return undefined
  return process.argv[index + 1]
}

const input = arg('--input')
const output = arg('--output')

if (!input || !output) {
  throw new Error('Usage: bun run render --input doctor.json --output report.pdf')
}

const report = JSON.parse(await readFile(input, 'utf8')) as Report
await renderToFile(<ReportDocument report={report} />, output)
console.log(`PDF report written to ${output}`)
