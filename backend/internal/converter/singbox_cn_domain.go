package converter

// defaultSingboxCNDomainSuffixes is a compact offline list safe for SFI/SFM.
// Full dnsmasq-china-list (~110k) inflates the subscription to multi‑MB and
// prevents mobile clients from starting. ".cn" covers the entire CN TLD tree.
func defaultSingboxCNDomainSuffixes() []string {
	return []string{
		"cn",
		"baidu.com", "bdstatic.com", "bdimg.com",
		"qq.com", "gtimg.com", "qpic.cn", "qcloud.com", "tencent.com", "weixin.qq.com",
		"taobao.com", "tmall.com", "alicdn.com", "aliyun.com", "alipay.com", "alibaba.com", "aliyuncs.com",
		"jd.com", "360buyimg.com",
		"bilibili.com", "hdslb.com", "bilivideo.com",
		"zhihu.com", "zhimg.com",
		"weibo.com", "sina.com.cn", "sinaimg.cn",
		"163.com", "126.com", "126.net", "netease.com",
		"iqiyi.com", "iqiyipic.com", "youku.com", "ykimg.com",
		"douyin.com", "bytedance.com", "byteimg.com", "toutiao.com", "snssdk.com",
		"mi.com", "xiaomi.com", "miui.com", "mi-img.com",
		"huawei.com", "hicloud.com", "dbankcdn.com",
		"meituan.com", "dianping.com", "dpfile.com",
		"pinduoduo.com", "yangkeduo.com",
		"kuaishou.com", "yximgs.com",
		"ele.me", "elemecdn.com",
		"amap.com", "autonavi.com", "gaode.com",
		"ctrip.com", "qunar.com", "12306.cn",
		"csdn.net", "gitee.com", "oschina.net",
		"douban.com", "doubanio.com",
		"ximalaya.com",
		"suning.com",
		"sohu.com", "sohucs.com", "ifeng.com",
		"cctv.com", "cntv.cn",
		"apple.com.cn", "icloud.com.cn", "mzstatic.com",
		"msn.cn", "cn.bing.com",
		"uc.cn", "ucweb.com", "sm.cn",
		"sogou.com",
		"upyun.com", "qiniucdn.com", "qiniudn.com",
		"ac.cn", "edu.cn", "gov.cn", "mil.cn", "org.cn", "com.cn", "net.cn",
	}
}
