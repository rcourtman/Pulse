const fs = require('fs');

const updateFile = (filepath) => {
  let code = fs.readFileSync(filepath, 'utf8');

  // Insert import if not present
  if (!code.includes('SparklineSkeleton')) {
    code = code.replace(
      `import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';`,
      `import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';\nimport { SparklineSkeleton } from '@/components/shared/SparklineSkeleton';`
    );
  }

  // Replace fallback
  const searchPattern = /<div class="text-sm text-slate-400 dark:text-slate-500 py-2">\s*\{\s*isCurrentRangeLoaded\(\) \? 'No history yet' \: 'Loading history\.\.\.'\s*\}\s*<\/div>/g;
  
  const replaceStr = `isCurrentRangeLoaded() ? (
                                            <div class="text-sm text-slate-400 dark:text-slate-500 py-2">
                                                No history yet
                                            </div>
                                        ) : (
                                            <SparklineSkeleton />
                                        )`;

  code = code.replace(searchPattern, replaceStr);

  fs.writeFileSync(filepath, code);
};

updateFile('./src/components/Infrastructure/InfrastructureSummary.tsx');
updateFile('./src/components/Workloads/WorkloadsSummary.tsx');

