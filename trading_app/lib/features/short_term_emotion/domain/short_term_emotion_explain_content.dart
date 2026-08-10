import 'short_term_emotion_explain_models.dart';

class ShortTermEmotionExplainContent {
  const ShortTermEmotionExplainContent._();

  static const score = ShortTermEmotionExplainPageData(
    title: '市场情绪分说明',
    subtitle: '综合分不是买卖信号，是短线环境温度计',
    summary:
        '市场情绪分把宽度、涨跌停质量、连板生态、异动强弱、板块主线和指数量能合成 0-100 分。它主要用于判断今天适不适合出手，以及应该主动还是防守。',
    sections: [
      ShortTermEmotionExplainSection(
        title: '分数怎么看',
        rows: [
          ShortTermEmotionExplainRow(
            label: '0-34',
            value: '冰点/退潮',
            note: '少做或不做，优先处理持仓风险。',
          ),
          ShortTermEmotionExplainRow(
            label: '35-44',
            value: '弱修复',
            note: '只看最强核心，普通机会容易冲高回落。',
          ),
          ShortTermEmotionExplainRow(
            label: '45-59',
            value: '分歧震荡',
            note: '可以观察，仓位要轻，避免追后排。',
          ),
          ShortTermEmotionExplainRow(
            label: '60-74',
            value: '活跃偏强',
            note: '短线可参与，但仍要看炸板和跌停风险。',
          ),
          ShortTermEmotionExplainRow(
            label: '75-100',
            value: '高潮/强一致',
            note: '赚钱效应强，但次日分歧风险也会上升。',
          ),
        ],
      ),
      ShortTermEmotionExplainSection(
        title: '情绪阶段',
        rows: [
          ShortTermEmotionExplainRow(
            label: '数据不足',
            value: '接口没有足够数据',
            note: '不输出交易倾向。',
          ),
          ShortTermEmotionExplainRow(
            label: '退潮防守',
            value: '跌停、空头异动或宽度明显恶化',
            note: '超短优先避坑。',
          ),
          ShortTermEmotionExplainRow(
            label: '活跃偏分歧',
            value: '有赚钱效应但风险信号没有消失',
            note: '适合轻仓试错。',
          ),
          ShortTermEmotionExplainRow(
            label: '强势活跃',
            value: '宽度、涨停、主线和量能共同支持',
            note: '可提高关注度，但仍要防一致后分歧。',
          ),
        ],
      ),
      ShortTermEmotionExplainSection(
        title: '仓位建议含义',
        body: '仓位只是风控参考，不代表必须买入。对超短来说，低分环境最重要的是少亏，高分环境也要避免情绪高潮后的后排接力。',
      ),
    ],
  );

  static const dashboard = ShortTermEmotionExplainPageData(
    title: '盯盘仪表盘说明',
    subtitle: '先看市场能不能做，再看个股值不值得做',
    summary: '盯盘仪表盘展示的是盘中最需要快速扫一眼的风险指标。它不追求解释所有行情，只帮你在下单前确认今天有没有明显短线陷阱。',
    sections: [
      ShortTermEmotionExplainSection(
        title: '核心指标',
        rows: [
          ShortTermEmotionExplainRow(
            label: '红盘率',
            value: '上涨家数 / 上涨下跌总数',
            note: '低于 40% 通常说明赚钱效应偏弱。',
          ),
          ShortTermEmotionExplainRow(
            label: '涨跌停',
            value: '涨停数 / 跌停数',
            note: '跌停明显增多时，超短接力风险会快速上升。',
          ),
          ShortTermEmotionExplainRow(
            label: '炸板率',
            value: '打开涨停板 / 封板数量',
            note: '炸板率高说明封板质量差，追板更容易吃面。',
          ),
          ShortTermEmotionExplainRow(
            label: '最高连板',
            value: '市场最高空间板',
            note: '空间越高，接力生态越活跃，但高位一致也可能次日分歧。',
          ),
          ShortTermEmotionExplainRow(
            label: '异动强弱',
            value: '多头异动 / 空头异动',
            note: '空头异动超过多头时，要降低进攻欲望。',
          ),
          ShortTermEmotionExplainRow(
            label: '指数量能',
            value: '两市成交额和量能分',
            note: '缩量上涨容易虚强，放量杀跌也要防风险释放。',
          ),
        ],
      ),
      ShortTermEmotionExplainSection(
        title: '超短使用方式',
        body: '开盘后先看红盘率和跌停，确认环境是否极端；再看炸板率和异动强弱，判断追高是否危险；最后看主线和量能，判断机会是否集中。',
      ),
    ],
  );

  static const components = ShortTermEmotionExplainPageData(
    title: '评分拆解说明',
    subtitle: '看清楚分数从哪里来，也看清楚哪里在拖后腿',
    summary:
        '评分拆解把总分拆成六个固定权重维度。它的价值不是追求绝对准确，而是让你知道当前市场到底是宽度好、涨停强、主线强，还是只是某一个指标短暂好看。',
    sections: [
      ShortTermEmotionExplainSection(
        title: '默认权重',
        rows: [
          ShortTermEmotionExplainRow(
            label: '宽度情绪',
            value: '25%',
            note: '看红盘率，判断赚钱效应是否扩散。',
          ),
          ShortTermEmotionExplainRow(
            label: '涨跌停质量',
            value: '25%',
            note: '看涨停、跌停、炸板率，是超短避坑的核心。',
          ),
          ShortTermEmotionExplainRow(
            label: '连板生态',
            value: '20%',
            note: '看最高连板和 3 板以上数量。',
          ),
          ShortTermEmotionExplainRow(
            label: '异动强弱',
            value: '15%',
            note: '看盘中多空异动谁更占优。',
          ),
          ShortTermEmotionExplainRow(
            label: '板块主线',
            value: '10%',
            note: '看有没有清晰主线承接资金。',
          ),
          ShortTermEmotionExplainRow(
            label: '指数量能',
            value: '5%',
            note: '看两市成交额是否支持行情延续。',
          ),
        ],
      ),
      ShortTermEmotionExplainSection(
        title: '为什么偏避坑',
        body: '模型会额外惩罚炸板率、跌停数量和空头异动。也就是说，就算某些指标好看，只要风险信号变重，总分和动作建议也会被压下来。',
      ),
    ],
  );
}
