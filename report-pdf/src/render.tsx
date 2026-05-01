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
  summaryBox: {
    flex: 1,
    padding: 12,
    borderRadius: 4,
    backgroundColor: '#f0f4f8',
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

function passedCount(report: Report): number {
  return report.checks.filter((check) => check.status === 'passed').length
}

function ReportDocument({ report }: { report: Report }) {
  const failed = report.checks.filter((check) => check.status === 'failed' && !check.ignored)
  const ignored = ignoredChecks(report)

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
            <Text>Profile: {report.profile.join(', ')}</Text>
            <Text>Passed checks: {passedCount(report)}</Text>
            <Text>Open findings: {failed.length}</Text>
            <Text>Ignored findings: {ignored.length}</Text>
          </View>
        </View>

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
