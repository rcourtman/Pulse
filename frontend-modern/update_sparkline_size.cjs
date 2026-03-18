const fs = require('fs');
const file = './src/components/shared/InteractiveSparkline.tsx';
let code = fs.readFileSync(file, 'utf8');

code = code.replace(
`  yMode?: 'percent' | 'auto';`,
`  yMode?: 'percent' | 'auto';
  size?: 'sm' | 'md' | 'lg';`
);

// update SVG stroke-width
code = code.replace(
`                      stroke-width={(() => {
                        const active = activeEmphasisSeriesIndex();
                        if (active === null) {
                          return '1.5';
                        }
                        return active === pathData.seriesIndex ? '2.8' : '0.9';
                      })()}`,
`                      stroke-width={(() => {
                        const active = activeEmphasisSeriesIndex();
                        const isLg = props.size === 'lg';
                        if (active === null) {
                          return isLg ? '2' : '1.5';
                        }
                        return active === pathData.seriesIndex ? (isLg ? '3.5' : '2.8') : (isLg ? '1' : '0.9');
                      })()}`
);

// update Canvas lineWidth
code = code.replace(
`      const lineWidth = active === null ? 1.5 : active === seriesIndex ? 2.8 : 0.9;`,
`      const isLg = props.size === 'lg';
      const lineWidth = active === null ? (isLg ? 2 : 1.5) : active === seriesIndex ? (isLg ? 3.5 : 2.8) : (isLg ? 1 : 0.9);`
);

fs.writeFileSync(file, code);
