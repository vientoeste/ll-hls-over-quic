# VOD 서비스를 위한 QUIC 기반 LL-HLS 개발

# 0. 요약
VOD 서비스 시장은 지속적으로 성장하여 사용자 경험을 개선할 필요가 요구되어 왔다. 기존의 HTTP 기반 스트리밍 방식은 세그먼트 단위 분할 및 전송으로 인한 본질적 지연과 하위 계층 전송 프로토콜인 TCP의 기술적인 한계로 빠른 시청 시작, 네트워크 변화에 대한 취약성 등의 문제를 겪고 있다. 본 논문은 이러한 사용자 경험을 개선하기 위한 방안으로 저지연 스트리밍 기술인 LL-HLS와 차세대 전송 프로토콜인 QUIC(HTTP/3)을 VOD 서비스에 결합하는 새로운 접근 방식을 제안하고 구현하였다. 이를 통해 아직 적용 사례가 많지 않은 QUIC 위의 LL-HLS(LL-HLS over QUIC) 시스템에 대한 기술적 타당성과 이에 대한 실제 구현 가능성을 입증한다.

# 서론
현대 사회에서 VOD(Video On Demand) 서비스는 주요 디지털 컨텐츠 소비 형태로 자리 잡으며 폭발적으로 성장하고 있다. 특히 사용자들의 요구 수준 상승에 따라 끊김 없는 영상 시청 경험, 즉각적 탐색, 빠른 재생 시작 등과 같은 사용자 경험의 개선이 서비스 경쟁력 확보에 필수적인 과제로 대두되었다. 이에 따라 영상 전송 기술은 Progressive Download 등의 방식에서 영상을 청크 단위로 나눠 전송하는 RTSP 등이 등장하고, 강력한 기존 인프라인 웹을 활용한 HTTP 기반 Adaptive Streaming(HLS, DASH 등)으로 발전했다. 하지만 주로 사용되는 HTTP 기반 스트리밍 기술들은 영상을 세그먼트 단위로 처리하여 전송하기 때문에 Startup Latency 등 본질적인 지연이 발생하게 된다. 스트리밍 서비스의 기반이 되는 기존 전송 프로토콜인 TCP와 위에서 동작하는 HTTP/1.1, HTTP/2는 3-Way Handshake와 같은 연결 설정 과정에서의 시간 소요, HOL Blocking(Head-Of-Line Blocking) 문제, 연결 환경 변화 시 필요로 하는 재연결 등 기술적 한계가 내포되어 있으며[^1] [^2] 이는 VOD 서비스의 스트리밍 성능과 안정성 저하의 원인이 될 수 있다.

이러한 문제를 해결하기 위해 응용 계층에서는 LL-HLS(Low-Latency HLS) 및 LL-DASH(Low-Latency DASH) 와 같은 저지연 스트리밍 기술이, 전송 계층에서는 TCP의 HOL Blocking을 근본적으로 해결하고 연결 설정을 최적화한 프로토콜 QUIC(Quick UDP Internet Connections)이 등장했다. 이 외에도 QUIC은 0~1 RTT(Round Trip Time)의 빠른 연결 설정, 그리고 Connection Migration 기능[^3] 등을 통해 VOD 서비스의 사용자 경험을 크게 개선할 잠재력을 지니고 있다. 그러나 오픈소스 기반의 LL-HLS가 제공하는 저지연 스트리밍의 이점과 QUIC(HTTP/3)이 제공하는 전송 효율성 및 연결 안정성 등을 VOD 서비스에 통합적으로 적용하려는 시도는 아직 초기 단계에 머무르고 있다. 특히 HTTP/3 기반의 오픈소스 LL-HLS 지원 서버 사례는 실험적으로 이를 지원하는 OvenMediaEngine을 제외하면 실제 구현 및 검증이 미흡한 실정이므로 해당 기술 조합의 실효성에 대한 심층적인 연구가 필요하다.

따라서 본 논문은 VOD 서비스 환경에 최적화된 QUIC 기반 HTTP/3 서버에서 LL-HLS 프로토콜을 사용해 컨텐츠를 효율적으로 전송하기 위한 서버를 직접 설계하고 구현한다. 이를 통해 VOD 서비스의 Startup Latency 감소를 통한 재생 시작 시간 단축, 컨텐츠 서빙의 네트워크 변화에 대한 내성 등을 개선하고 향후 차세대 고성능 VOD 스트리밍 시스템 구축에 기여할 것을 기대한다.

# 이론적 배경 및 관련 연구

## 비디오 전송 기술

VOD 서비스가 대중화되기 시작하며 초기에는 당시의 네트워크 환경과 기술 수준 등을 반영한 다양한 비디오 전송 기술들이 발전 및 사용되었다. 초기 스트리밍 방법은 적응력이 부족하여 버퍼링과 저품질 재생과 같은 문제가 발생했으며 시간이 지남에 따라 변화하는 네트워크 조건에 따라 비디오 품질을 동적으로 조정하는 적응형 비트레이트 알고리즘(ABR)을 활용하는 기술이 발달했다[^4]. 본 절에서는 다운로드 후 재생 방식부터 시작하여 ABR 기반 스트리밍 기술에 이르기까지의 비디오 전송 기술의 역사적 발전 과정을 추적한다.

### 기존 주요 비디오 전송 기술의 발전

#### 비디오 다운로드

초기에 인터넷을 통해 영상 컨텐츠를 전송 및 시청하기 위해서 비디오 파일 전체를 사용자 기기에 완전히 다운로드한 뒤 재생하는 방식이 FTP 또는 HTTP 위에서 사용되었다[^1]. FTP(File Transfer Protocol)는 1971년 RFC 114로 제안되어 추후 RFC 959에서 인터넷 표준으로 자리매김한 프로토콜로, 영상 컨텐츠 시청을 위해서는 해당 프로토콜 등을 통해 전체 비디오 파일을 다운로드한 후 로컬에서 재생하는 방식으로 활용했다[^5]. 이는 재생 전 파일을 다운로드하는 데 상당한 시간이 소요되었으며 데이터가 평문으로 전송되어 공격에 취약했고 대용량 파일 전송으로 인해 단일 서버가 장시간 특정 요청에 점유될 수 있는 등의 단점이 있었다[^5]. 뿐만 아니라 다운로드 중 특정 부분으로 이동하여 재생(seeking)하거나 네트워크 상태에 따라 화질을 조절하는 등의 스트리밍 관련 기능이 전무했다[^5]. 1990년대 중반 HTTP가 등장하며 FTP를 통한 다운로드 후 재생 방식의 단점을 일부 개선했지만 본격적인 스트리밍 환경을 구축하는 데에는 여러 한계가 존재했다. HTTP 사용 시 방화벽 문제 개선 및 폭발적으로 성장하던 웹(서버 및 브라우저) 인프라를 그대로 활용 가능함에 따라 편의성 및 비용 절감 등의 장점들이 있지만 앞서 언급한 다운로드 후 재생 방식의 단점들은 크게 개선되지 않았다.

앞서 설명한 단점들을 극복하기 위해 HTTP 프로토콜의 발전에 따라 비디오 등 미디어 파일의 앞부분부터 순차적으로 다운로드하면서 재생이 가능하게 하는 Progressive Download 방식이 등장했다. 이는 서버에 저장된 미디어 파일을 내려받는 동시에, 클라이언트의 미디어 플레이어는 수신된 데이터가 일정량 이상 버퍼링되면 즉시 재생을 시작하는 방식으로 동작한다. 특히 1990년대 중반, 주로 다이얼업 모뎀이나 초기 ISDN 환경에서 인터넷 대역폭이 현재보다 훨씬 제한적이고 불안정했을 때 사용자는 파일 전체가 다운로드될 때까지 기다릴 필요 없이 미디어 콘텐츠를 보다 빠르게 접할 수 있게 되어 사용자 경험이 크게 향상되었다[^5]. 하지만 이는 네트워크 대역폭 변화에 적응하여 화질을 동적으로 변경하는 기능이 부재했고[^4] 사용자의 능동적 탐색 지원이 미흡했으며, 한 번 재생이 시작되면 사용자의 네트워크 상태가 나빠져도 동일한 품질로 계속 다운로드를 시도하여 버퍼링이 길어지거나 재생이 중단될 수 있는 단점이 여전히 존재했다[^2].

이러한 비디오 다운로드 방식의 한계와 더불어 라이브 이벤트 중계와 같은 실시간성에 대한 요구가 커지면서 단순 다운로드 방식으로만 서비스를 구성하기 어려워졌다. 이에 따라 각종 스트리밍 프로토콜들이 등장하여 실시간 데이터 전송과 제어 기능을 제공하려는 시도들이 이루어졌다[^1].


#### 비디오 스트리밍

비디오 스트리밍은 데이터를 작은 단위의 패킷으로 분할하여 연속적인 흐름인 스트림 형태로 전송하고, 클라이언트는 이를 수신하는 즉시 버퍼링 후 디코딩하여 재생함으로써 대기 시간을 최소화하는 것을 목표로 한다. 이에 초기 스트리밍 기술은 주로 실시간 전송과 재생 제어에 초점을 맞추어 발전했다. 특히 1990년대 중반에는 독자적인 실시간 스트리밍 기술들이 등장하기 시작했다. 1995년 4월 RealNetwork 사에서 출시한 RealAudio는 독자적으로 개발한 당시로서 획기적인 실시간 오디오 스트리밍 기술을 선보여 주목을 받았으며 Microsoft 사에서는 이에 대응하여 NetShow를 1996년에 발표하며 스트리밍 시장에 진출했다. NetShow는 이후 Windows Media Services로 발전하였고, TCP 또는 UDP를 통해 유니캐스트 데이터를 전송하는 독자적인 MMS(Microsoft Media Server) 프로토콜을 사용하여 미디어를 스트리밍했다.

한편, 특정 기업에 종속되지 않는 표준화된 스트리밍 프로토콜이 IETF(Internet Engineering Task Force)를 중심으로 개발이 진행되었다. 1996년 처음 제안된 RTP(RFC 3550)는 오디오 및 비디오와 같은 실시간 데이터를 IP 네트워크를 통해 전송하기 위한 종단 간 전송 프로토콜이다. 이는 데이터의 시간 정보를 포함한 지터 보상(수신 측에서 패킷 도착 시간의 불규칙성을 보정하는 기능) 및 패킷 손실 감지 등을 지원하지만 데이터 전송 자체의 신뢰성을 보장하거나 지연시간을 제어하지 않는다. 이러한 RTP를 제어하는 프로토콜로 1998년 RFC 2326으로 RTSP(Real Time Streaming Protocol)가 표준화되었다. RTSP는 멀티미디어 서버에 대한 네트워크 원격 제어 역할을 수행하는 응용 계층 프로토콜로 각종 명령을 통해 데이터 전송을 제어하며 이는 별도의 제어 채널을 통해 스트림의 재생, 중지, 탐색을 관리하고 실제 미디어 데이터는 RTP를 통해 전송하고 하는 구조로 이뤄진다[^6]. 비슷한 시기에 Macromedia 사(추후 Adobe 사에서 인수)가 개발한 RTMP(Real-Time Messaging Protocol)도 널리 사용되었다. RTMP는 Adobe Flash Player와 서버 간 오디오, 비디오, 데이터 스트리밍을 위해 설계된 프로토콜로, TCP 기반의 단일 연결을 통해 제어 메시지와 미디어 데이터를 함께 전송하여 당시 HTTP 기반의 연결 방식보다 낮은 지연 시간을 가졌다. 그러나 Adobe Flash Player의 보안 문제와 비표준 기술 의존성으로 인해 점차 웹에서 도태되었고 현재는 주로 인코더에서 서버로 영상을 전송하는 초기 단계에서 제한적으로 사용되고 있다.

이러한 초기 및 기존 비디오 전송 기술들은 각각의 장점과 함께 명확한 한계점을 가지고 있었으며, 이는 특히 HTTP 인프라의 광범위한 보급과 함께 웹 환경에 더 친화적이고 네트워크 변화에 유연하게 대응할 수 있는 새로운 스트리밍 기술의 필요성을 증대시켰다. 한편, WebRTC와 같은 실시간 통신 기술은 화상 회의나 음성 통화와 같은 양방향, 초저지연 시나리오에 최적화되어 개발되었으나 단방향 스트리밍이나 VOD 서비스에는 적합하지 않다. WebRTC는 UDP 기반 RTP를 활용해 실시간성을 보장하지만, 신뢰성 있는 전송, 대규모 사용자 지원, CDN 활용 등 VOD의 핵심 요구사항을 충족하기 어렵다. 따라서 본 논문은 VOD 서비스에 최적화된 저지연 스트리밍을 위해 HTTP 기반의 LL-HLS와 UDP를 기반으로 하되 TCP의 안정성을 계승한 QUIC(HTTP/3)을 결합하는 접근법에 초점을 맞춘다.

### HLS와 LL-HLS

앞서 언급된 기존 비디오 전송 기술들의 한계를 극복하기 위해 널리 사용되고 있는 웹 인프라를 효과적으로 활용하는 HTTP 기반 적응형 스트리밍(Adaptive Bitrate Streaming over HTTP) 기술이 등장했다. ABR(Adaptive Bitrate) 알고리즘은 클라이언트의 네트워크 환경, CPU 사용량 등 지속적으로 변화하는 환경을 모니터링하여 가장 적절한 품질의 콘텐츠를 전송하도록 한다[^4]. 대표적인 ABR 스트리밍 기술로는 Apple 사에서 개발하고 RFC로 표준화된 HLS(HTTP Live Streaming), ISO/IEC 국제 표준인 MPEG-DASH(Dynamic Adaptive Streaming over HTTP) 등이 있다. Apple 사에서 2009년에 발표한 HLS는 HTTP 기반으로 동작하는 적응형 비트레이트 스트리밍 프로토콜로, 원본 비디오를 여러 화질로 세분화하고 짧은 길이로 자른 세그먼트 파일들과 세그먼트들의 목록 및 메타데이터를 담은 매니페스트 파일로 재구성하여[^7] [^8] 클라이언트의 네트워크 상태에 따라 적절한 화질의 세그먼트를 전송하여 최대한 끊김 없는 시청 경험을 제공하려고 노력한다[^9]. HLS는 HTTP에서 동작하기 때문에 방화벽 문제로부터 보다 자유롭고 CDN(Content Delivery Network)을 활용한 효율적인 콘텐츠 전송이 가능하여 대규모 스트리밍 서비스 구축에 유리하다[^10]. MPEG-DASH는 HLS와 유사하게 미디어를 세그먼트로 분할하고 매니페스트 파일인 MPD(Media Presentation Description)를 통해 클라이언트에게 다양한 품질의 스트림 정보를 제공한다. 이는 codec-agnostic한 특징을 가졌으며 공통 암호화와 같은 DRM(Digital Rights Management) 표준과의 연동성 또한 고려되어 있다. 본 논문에서 제안하는 시스템은 HLS를 기반으로 구현되었으므로 이후 HLS를 중심으로 기술한다.

HLS 및 MPEG-DASH는 안정적인 스트리밍 환경을 제공하는 데 크게 기여했지만 라이브 스트리밍처럼 실시간 상호작용이 중요한 서비스에서는 높은 지연 시간이 문제가 되었다. 수십 초에 달하기도 하는 지연 시간은 주로 세그먼트 단위로 미디어를 처리하고 버퍼링하는 방식 자체에서 발생한다. 라이브 스트리밍 환경에서는 종단 간 지연을 최소화하는 것이 매우 중요하기 때문에 라이브 HTTP 스트리밍 서비스는 낮은 네트워크 지연 시간뿐만 아니라 클라이언트의 재생 버퍼 길이 또한 짧게 유지해야 하는 특징이 있다[^2]. 하지만 기존 HTTP 스트리밍 방식은 안정성을 위해 버퍼를 길게 유지하는 경향이 있어 라이브 스트리밍 서비스에는 부적합하며, 이러한 문제를 해결하기 위해 기존 HLS와 MPEG-DASH를 확장해 지연 시간을 수 초 이내로 단축하는 저지연 스트리밍 기술인 LL-HLS 및 LL-DASH가 등장했다. LL-HLS는 부분 세그먼트(Partial Segments) 전송, Delta Playlist Update, Blocking Playlist Reload, Preload Hint 등의 기술을 도입해 세그먼트 전체가 완성되기 전에도 미디어 청크를 전송하고 플레이어가 빠르게 이를 로드할 수 있도록 하여 지연 시간을 크게 단축시켰다.

## HTTP

HTTP는 월드 와이드 웹에서 정보를 주고받기 위한 핵심 응용 계층 프로토콜로, 초기의 HTTP/0.9부터 HTTP/1.0, HTTP/1.1을 거쳐 HTTP/2, 그리고 최근의 HTTP/3에 이르기까지 지속적으로 발전해왔다. HTTP/3는 모든 상황에서 기존 프로토콜들과 대비하여 성능 향상을 보이지는 않지만 특정 네트워크 조건, 특히 높은 지연 시간이나 낮은 대역폭 환경에서는 상당한 성능 이점을 보인다[^11] [^3]. 이를 통해 네트워크 환경이 좋지 않은 모바일 VOD 서비스 등에 HTTP/3(QUIC) 결합 시 사용자 경험을 향상시킬 것을 기대한다. 본 절에서는 이에 대해 HTTP의 발전과 함께 살펴본다. 

### SPDY, HTTP/2

HTTP/1.1은 오랜 기간 동안 웹 통신의 기반이 되었지만 몇 가지 성능 상 한계가 존재했다. 대표적으로 하나의 TCP 연결에서 한 번에 하나의 요청과 응답만을 처리하는 순차적 처리 방식, 비교적 큰 텍스트 기반 헤더 및 HOL(Head-Of-Line) 블로킹 등이 문제점으로 지적되었다. 이러한 한계를 극복하기 위해 Google 사에서 2009년 SPDY라는 실험적 프로토콜을 개발했다. SPDY는 단일 TCP 연결 상에서 여러 요청을 동시에 처리할 수 있는 다중화(Multiplexing), 요청 우선순위 지정, 헤더 압축(HPACK) 등의 기능을 도입하여 웹 페이지 로딩 속도를 개선하고자 했다. 즉 SPDY는 HTTP를 대체하려는 프로토콜이 아닌 HTTP의 의미론을 유지하며 전송 방식을 개선하는 형태로 제안되었다. 이를 바탕으로 IETF HTTPbis 워킹 그룹은 HTTP/2 표준화 작업을 진행해 2015년 공식적으로 RFC 7540으로 HTTP/2를 발표했다. 이는 하나의 TCP 연결을 통해 여러 요청과 응답을 동시에 양방향으로 스트림 형태로 주고받을 수 있게 되어 응용 계층에서의 HOL 블로킹 문제를 완화하였다. 그러나 HTTP/2는 여전히 전송 계층 프로토콜로 TCP를 사용하기 때문에 TCP 계층에서의 HOL 블로킹 문제는 해결하지 못 했다[^3].

### QUIC, HTTP/3

전송 계층 프로토콜인 TCP에 의해 발생하는 HOL 블로킹 문제를 해결하기 위해 UDP 기반의 전송 프로토콜인 QUIC(Quick UDP Internet Connections)이 개발되었고 QUIC 위에서 동작하는 HTTP/3이 등장하게 되었다[^11]. QUIC은 TCP와 같은 기존 전송 프로토콜 대비 HTTP 트래픽을 가속화하고 HTTP/2와 같은 다중화 및 흐름 제어, TLS(Transport Layer Security)와 동등한 보안, TCP와 동일한 안정성 및 혼잡 제어를 제공하는 것을 목표로 한다. 이는 2012년 Jim Roskind에 의해 디자인되어 실험이 진행되었고 이후 IETF에서 표준화 작업을 거쳐 RFC 9000 등으로 발표되었다. 이외에도 QUIC은 TCP의 3-way handshake와 TLS handshake 과정을 통합하여 이전에 서버와 통신한 적이 없는 경우에는 1-RTT(Round Trip Time), 통신한 적이 있는 경우 0-RTT 만에 연결 설정을 완료하며 스트림을 다중화해 데이터를 병렬로 전송할 수 있다[^11]. 또한 한 스트림에서 데이터가 지연되거나 손실되어도 다른 스트림의 데이터 전송에 영향을 미치지 않아 TCP에서의 HOL blocking 문제를 해결했으며 클라이언트의 IP 주소나 포트가 변경되어도 기존 연결을 중단 없이 유지할 수 있도록 지원한다[^3].

HTTP/3는 QUIC 위에서 동작하도록 설계된 버전으로 RFC 9114로 표준화되었다. IETF는 인터넷 전송 프로토콜을 TCP에서 QUIC으로 변경하는 새로운 HTTP/3 초안을 2019년 1월에 발표한 바 있다. HTTP/3는 HTTP/2의 주요 의미론적 기능과 스트림 단위 흐름 제어 등을 계승하면서, 전송 계층으로 QUIC을 사용함으로써 TCP 기반 HTTP/2가 가졌던 한계점들을 극복하고자 한다. 이는 스트림 간 독립성을 보장하여 HOL 블로킹을 해결하고, 효율적인 혼잡 제어 및 빠른 연결 설정을 통해 TCP 기반 HTTP 대비 향상된 성능을 제공한다. 이처럼 HTTP/3(QUIC)는 TCP 기반 전송 프로토콜의 한계를 극복하고 저지연 스트리밍 환경 구축에 도움이 될 빠른 연결 수립, HOL 블로킹 없는 다중화 등의 기능을 제공한다. 이러한 특성들은 저지연을 목표로 하는 LL-HLS와 결합될 때 VOD 서비스의 초기 로딩 시간 단축, 재생 중 끊김 감소, 네트워크 환경 변화에 대한 대응력 향상 등 사용자 경험을 전반적으로 향상시킬 것으로 기대된다.

# LL-HLS over QUIC(HTTP/3) 개발

본 장에서는 서버의 구현 및 이에 대한 신뢰성, 재현성, 타당성 및 명세 만족성을 확보하기 위해 RFC 8216에서 요구하는 HLS의 기본 요구사항부터 LL-HLS 확장 명세의 핵심 기능까지 각 기능이 어떤 태그와 속성을 통해 구현되었는지 등에 대해 기술한다.

## 개발 환경

본 절에서는 QUIC 기반 LL-HLS 시스템 개발에 사용된 소프트웨어 및 하드웨어에 대해 기술한다.

### 소프트웨어 명세

QUIC 위의 LL-HLS 서버는 Go 언어 1.23 버전을 기반으로 개발되었으며, QUIC 프로토콜 및 HTTP/3 통신 스택은 quic-go 라이브러리 0.51.0 버전을 활용하여 구현하였다. 서비스에서 제공할 원본 미디어 컨텐츠는 FFmpeg을 사용하여 표준 LL-HLS 명세에서 요구하는 CMAF fMP4 포맷으로 인코딩 및 분할 처리하도록 설계했다. 클라이언트 측에서는 hls.js 플레이어를 사용하여 개발된 스트림의 동작을 검증하였다.

### 하드웨어 명세

운영 환경 서버는 AWS EC2를 사용, 개발 환경 서버는 Intel Core i5 8세대 CPU, 16GB 메모리 환경의 노트북 Lenovo Thinkpad X390에 구축하였다. 저장장치는 서비스 확장성을 고려해 운영 환경에서는 AWS S3를, 개발 환경에서는 내장 SSD를 사용했으며 운영체제는 모두 Ubuntu 24.04 LTS를 사용하였다. 네트워크 병목 현상을 최소화하고 프로토콜 자체의 성능을 보다 정확히 측정하기 위해 서버와 클라이언트 모두 1Gbps의 유선 네트워크 환경을 구축하였다.

## LL-HLS 지원 서버 구현의 기술적 과제

표준 LL-HLS 명세를 준수하는 오리진 서버를 구축하는 것은 표준 미디어 도구를 사용하는 것으로는 불충분하다. LL-HLS 명세는 클라이언트와의 동적 상호작용을 통해 지연 시간을 최소화하는 서버의 동작을 규정하는 반면 FFmpeg와 같은 표준 미디어 도구들은 미디어 변환, 즉 인코딩과 정적 파일 생성 등에 중점을 두기 때문이다. 즉 LL-HLS의 핵심인 동적 재생목록 업데이트(Dynamic Playlist Update), Server Push를 통한 선제적 리소스 전송 등의 기능은 FFmpeg의 지원 범위에 포함되지 않는다. 본 논문에서는 이러한 기술적 과제를 해결하기 위해 강력한 미디어 처리 도구인 FFmpeg를 기반으로 VOD 서비스의 파일을 실시간으로 분할 및 인코딩하고, Go 언어와 quic-go 라이브러리를 사용해 LL-HLS 명세의 핵심 기능들을 직접 구현한 오리진 서버를 설계하고 구축하여 LL-HLS가 목표하는 저지연 스트리밍을 달성하는 것을 목표한다.

## 시스템 아키텍처

본 시스템은 main 함수에서 설정 파일을 로드한 후, FFmpeg 트랜스코딩 프로세스를 비동기적으로 실행하고 HTTP 서버를 구동하는 구조로 이루어져 있다. 전체 아키텍처는 크게 미디어 처리 계층, 상태 관리 계층, HTTP 처리 계층의 세 부분으로 나뉜다.

### 미디어 처리 계층

미디어 처리 계층은 pkg/media에 위치하며 FFmpeg를 래핑(wrapping)하여 원본 비디오 파일을 LL-HLS 스트림으로 트랜스코딩하는 역할을 담당한다. GenerateLLHLS 함수는 서버 시작 시 또는 새로운 비디오 요청 시 호출되어, 지정된 비디오 소스를 fMP4 컨테이너 형식의 부분 세그먼트(.m4s)와 초기화 세그먼트(init.mp4)로 변환한다.

### 상태 관리 계층

상태 관리 계층은 pkg/hls, internal/models에 위치하며 생성된 미디어 파일들을 감시하고 스트림의 상태를 관리한다. Manager는 각 스트림의 상태(StreamState)를 관리하며 fsnotify 라이브러리를 사용해 미디어 파일 디렉터리를 감시한다. 새로운 부분 세그먼트 파일이 생성되면 이를 감지하고 해당 스트림의 StreamState를 갱신한다. StreamState에는 스트림의 미디어 시퀀스 번호, 파트 시퀀스 번호, 세그먼트와 파트 목록 등 재생목록 생성에 필요한 모든 정보를 저장한다. 또한 Go의 채널 updateChan을 이용해 상태 변경이 발생했음을 HTTP 처리 계층에 알리는 역할을 한다.

### HTTP 처리 계층

HTTP 처리 계층은 pkg/server, pkg/hls에 위치하며 클라이언트의 HTTP 요청을 받아 처리한다. Server는 quic-go 라이브러리를 사용하여 HTTP/3 (QUIC) 또는 HTTP/2 서버를 시작하고, chi 라우터를 이용해 /live/{streamID}/* 형태의 URL 요청을 HLS Handler로 전달한다. Handler는 실제 LL-HLS 로직이 구현된 부분으로, .m3u8 재생목록 요청과 .m4s/.mp4 미디어 파일 요청을 구분하여 처리한다. 특히 servePlaylist 함수는 상태 관리자로부터 현재 스트림 정보를 받아 동적으로 재생목록을 생성하고, LL-HLS의 블로킹 요청(Blocking Request)을 처리하는 핵심 로직을 포함한다.

이처럼 각 계층이 유기적으로 연동하여 FFmpeg가 정적으로 생성한 미디어 파일들을 기반으로 동적인 LL-HLS 스트리밍 서비스를 제공한다.

## LL-HLS 핵심 기능 구현

본 절에서는 시스템 아키텍처의 각 컴포넌트가 어떻게 상호작용하여 LL-HLS의 핵심 기능들을 구현하는지, 그리고 이 과정에서 사용된 구체적인 명령어와 코드, 그리고 HLS 태그들을 상세히 설명한다.

### 부분 세그먼트(Partial Segments)

LL-HLS는 기존 세그먼트를 더 잘게 나눈 전송 단위인 파트(part)를 정의해 클라이언트의 빠른 미디어 재생 시작을 돕고 지연 시간을 줄이는 부분 세그먼트를 사용한다. 이는 세그먼트 전체가 완성되기 전에 클라이언트가 수신 즉시 재생을 시작할 수 있도록 하며, LL-HLS 명세의 EXT-X-PART-INF와 EXT-X-PART 태그를 통해 구현된다. 특히 EXT-X-PART 태그의 INDEPENDENT=YES 속성을 통해 해당 파트가 I-frame으로 시작해 독립적으로 디코딩 가능함을 명시하여 플레이어가 파트 단위로 스트림을 안정적으로 시작하고 탐색할 수 있도록 한다. I-frame은 Intra-coded frame의 준말로 이전 프레임(P-frame)이나 이후 프레임(B-frame)의 정보 없이 독립적으로 완전한 이미지 정보 하나를 모두 담고 있는 비디오 프레임을 의미한다. 한편 P-frame은 직전의 I-frame이나 P-frame과의 차이만을 담으며 B-frame은 이전 프레임과 이후 프레임을 모두 참조해 그 사이의 변화만을 기록한다. 이는 영상 속도 조절이나 되감기 등의 기능을 구현하는 데 사용된다. P-frame과 B-frame을 사용하지 않고 I-frame으로만 미디어를 구성하면 영상의 모든 부분에 대해 빠른 탐색이 가능하지만 파일 사이즈가 매우 큰 폭으로 증가하기 때문에 VOD 서비스에서는 주로 세 종류의 프레임을 모두 사용하는 GOP 구조를 사용하며 I-frame만을 일정 간격으로 배치한 낮은 화질의 재생목록을 통해 탐색 시 미리보기를 제공한다.

이를 구현하기 위해 CMAF 규격을 준수하는 fMP4 컨테이너로 원본 미디어를 트랜스코딩해야 한다. 본 논문에서는 FFmpeg를 사용하며 핵심 명령어는 다음과 같다.

```go
cmd := exec.Command("ffmpeg",
    "-re", "-i", sourceFile,
    "-hide_banner", "-y",

    // 비디오, 오디오 인코딩 옵션들 생략
    
    "-f", "hls",
    "-hls_segment_type", "fmp4",
    "-hls_segment_filename", filepath.Join(outputDir, "seg%d.m4s"),
    "-hls_fmp4_init_filename", "init.mp4",
    "-hls_playlist_type", "event",
    "-hls_flags", "independent_segments",
    "-lhls", "1",
    "-hls_time", "0.5",
    playlistPath,
)
```

-f hls는 출력물을 HLS 스트리밍 형식으로 생성하도록 지정하며, -hls_time은 전체 미디어 세그먼트 목표 길이를 2초로 설정하고 EXTINF 태그 값의 기준으로 삼는다. -hls_fmp4_init_filename는 모든 세그먼트에 필요한 초기화 정보를 담는 파일(변환된_동영상_이름.mp4)을 생성하여 EXT-X-MAP 태그의 기반을 마련한다. 또한 -hls_flags independent_segments+omit_endlist를 통해 각 세그먼트가 독립적으로 디코딩 가능하도록 I-frame으로 시작하게 설정하며, -frag_duration을 0.5로 설정하여 부분 세그먼트의 크기를 0.5초 길이로 생성한다. 생성된 재생목록에는 다음과 같이 파트 정보가 포함된다.

```text
#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:2
#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=1.50,CAN-SKIP-UNTIL=12.00
#EXT-X-PART-INF:PART-TARGET=0.50000
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.000,
#EXT-X-PART:DURATION=0.500,URI="1080p/seg0.m4s",INDEPENDENT=YES
#EXT-X-PART:DURATION=0.500,URI="1080p/seg1.m4s"
#EXT-X-PART:DURATION=0.500,URI="1080p/seg2.m4s"
```

### Delta Update & Blocking 

Delta Update는 클라이언트가 전체 미디어 플레이리스트를 매번 다시 다운로드하는 대신 마지막으로 수신한 버전 이후의 변경사항만 요청하고 서버는 해당 부분만 응답하여 업데이터 지연과 서버 부하를 줄이는 LL-HLS 기능이다. 클라이언트는 쿼리 파라미터에 _HLS_msn과 _HLS_part를 작성하여 자신이 가진 미디어의 버전을 알리고 서버는 변경분(Delta)만 응답한다. 요청한 정보가 아직 없다면 서버는 응답을 잠시 보류(Blocking Request)하고 해당 정보가 생성되는 즉시 응답을 재개하여 불필요한 반복 요청을 줄여 지연 시간 단축에 직접적으로 기여하며 특히 저품질 네트워크 환경의 클라이언트 부담을 줄인다. Blocking Request는 EXT-X-SERVER-CONTROL 태그의 CAN-BLOCK-RELOAD=YES 속성 및 EXT-X-SKIP 태그를 통해 작동한다.

본 논문에서는 해당 기능을 Go 언어의 동시성 기능을 활용해 효율적으로 구현한다. 다음은 Blocking Request를 처리하는 요청 핸들러의 핵심 로직을 나타낸 의사 코드이다.

```go
func (h *Handler) servePlaylist(w http.ResponseWriter, r *http.Request, streamID, rendition string) {
	streamState := h.manager.GetOrCreateStream(streamID, rendition)
	playlistData := streamState.Playlist()

	// 1. 쿼리 파라미터에서 클라이언트의 미디어 버전 파싱
	clientMSNStr := r.URL.Query().Get("_HLS_msn")
	clientPartStr := r.URL.Query().Get("_HLS_part")

	if clientMSNStr != "" && clientPartStr != "" {
		// 클라이언트 버전 파싱 로직 생략

		// 2. 클라이언트가 최신 버전을 가지고 있는지 확인
		isClientUpToDate := false
		if errMSN == nil && errPart == nil && playlistData.LastPart != nil {
			// 최신 버전 확인 로직 생략
			isClientUpToDate = true
		}

		// 3. 최신 버전이면 Blocking Request 처리
		if isClientUpToDate {
            // 상태 변경 알림 채널 구독
			updateChan := streamState.SubscribeToUpdates()
			ctx := r.Context()
			select {
            //새로운 파트 생성 신호 수신 및 대기 종료
			case <-updateChan: 
            // 타임아웃 및 대기 종료
			case <-time.After(5 * time.Second):
            // 클라이언트 연결 종료
			case <-ctx.Done():
				return
			}
		}
	}
	// 4. 최신 재생목록 생성 및 전송
	playlist, err := h.generatePlaylist(r, &playlistData)
	w.Write([]byte(playlist))
}
```

해당 과정에서 클라이언트의 요청은 HTTP/3 요청 핸들러의 PlaylistHandler 함수에서 처리되며 이는 세션 관리자를 통해 현재 미디어 상태를 확인하고 재생목록 생성기 로직을 수행하는 함수 GenerateDeltaPlaylist를 호출한다. 이 때 응답을 즉시 보낼 수 없는 경우 Go 채널인 updateChan을 통해 파일 시스템 관리자로부터의 업데이트 신호를 비동기적으로 기다린다. 이러한 Go의 동시성 기능을 활용한 구현은 불필요한 반복 요청(Polling)을 제거하고 여러 클라이언트의 요청을 적은 리소스로 효율적으로 처리할 수 있게 한다. 이는 추후 서비스 확장 시, AWS S3 등의 외부 저장소를 사용하게 될 것으로 파일 시스템 관리자를 다른 마이크로서비스의 업데이트 알림으로 대체할 예정이다.     

### 선제적 로딩(Preload Hint & Server Push)

Preload Hint는 클라이언트의 다음 요청을 예측하고 해당 리소스를 HTTP/2 등에서 지원하는 Server Push 기능을 사용해 선제적으로 전송해 요청-응답에 필요한 왕복 시간을 제거하는 것을 목표하는 기능이다. 클라이언트의 다음 요청을 유도하는 HINT의 경우 1-RTT를 절약하는 반면 Server Push는 이미 사용 중인 스트림으로 이를 전송한다. LL-HLS 명세에서는 EXT-X-PRELOAD-HINT 태그로 다음에 필요할 리소스를 클라이언트에게 암시한다.

```go
// pkg/hls/handler.go의 generatePlaylist 함수 내부
// Preload Hint - Server Push
if playlistData.LastPart != nil {
    // 다음 파트의 URI를 예측하는 로직 생략
    // e.g. nextPartURI := "1080p/seg_001.mp4.part3.m4s"

    fmt.Fprintf(&sb, "#EXT-X-PRELOAD-HINT:TYPE=PART,URI=\"%s\"\n", nextPartURI)

    // Server Push 시도
    if pusher, ok := r.Context().Value(http.ServerContextKey).(http.Pusher); ok {
        err := pusher.Push(nextPartURI, nil)
        if err != nil {
            log.Printf("Failed to push %s: %v", nextPartURI, err)
        }
    }
}
```

generatePlaylist 함수는 재생목록을 생성할 때 마지막으로 추가된 파트 정보를 기반으로 다음 파트의 URI를 예측한다. 예측된 URI는 #EXT-X-PRELOAD-HINT 태그에 포함되어 클라이언트에게 전달된다. 동시에, 현재 연결이 HTTP/2 또는 HTTP/3를 사용하고 서버 푸시를 지원하는 경우(http.Pusher 인터페이스 확인), 예측된 리소스를 서버 푸시를 통해 클라이언트에게 선제적으로 전송하여 요청-응답 왕복 시간(RTT)을 제거하고 지연 시간을 추가로 단축한다.

### 상태 보고(Rendition Report)와 적응형 스트리밍(ABR)

LL-HLS는 다양한 네트워크 환경의 클라이언트를 고려해 적응형 스트리밍 기반을 마련하여 부드러운 화질 전환을 지원하는 것을 목표로 한다. 이는 마스터 재생목록의 EXT-X-STREAM-INF 태그와 미디어 재생목록의 EXT-X-RENDITION-REPORT 태그를 통해 구현된다. ABR의 기본 원리는 동일한 컨텐츠를 여러 품질(화질, 비트레이트)의 스트림, 즉 렌디션(Rendition)으로 제공하는 것이다. 클라이언트 플레이어 hls.js는 자체적으로 자신의 네트워크 대역폭과 버퍼 상태를 지속적으로 측정하여 현재 상황에 가장 적합한 렌디션을 선택해 서버에 요청한다. 본 논문의 서버에서는 이 렌디션들의 목록을 마스터 재생목록에 포함시켜 클라이언트에게 제공한다. EXT-X-STREAM-INF 태그는 각 렌디션의 최대 대역폭(Bandwidth), 해상도(Resolution), 사용된 코덱 등의 정보를 담아 플레이어가 가장 적합한 환경에서 컨텐츠를 시청할 수 있도록 돕는다.

전통적인 HLS 환경에서는 세그먼트의 길이가 수 초에 달해 플레이어가 다른 렌디션으로 전환할 때 해당 세그먼트가 서버에 존재할 확률이 높았다. 하지만 파트 단위로 정보가 빠르게 갱신되는 LL-HLS 환경에서는 시청하던 환경의 스트림의 최신 파트 번호가 다른 스트림에서도 동일하게 사용 가능하다는 보장이 없다. 이는 각 렌디션의 인코딩 속도가 달라 파트 번호 동기화가 어긋날 수 있기 때문이다. 이러한 문제를 해결하기 위해 EXT-X-RENDITION-REPORT 태그에 특정 미디어 재생목록 안에 다른 렌디션의 가장 최신 미디어 시퀀스 번호(LAST-MSN 속성 사용)와 파트 번호(LAST-PART 속성 사용) 정보를 담게 한다. 따라서 클라이언트 플레이어는 현재 재생 중인 스트림의 재생목록만으로 다른 스트림들의 생성 현황을 알 수 있게 되어 전환하려는 렌디션에 반드시 존재하는 파트를 정확히 알고 요청할 수 있어 지연이나 오류 없이 부드러운 품질 전환을 수행할 수 있다. 이를 위해 마스터 재생목록 생성 로직을 분리해서 구현해야 한다.

```text
#EXTM3U
#EXT-X-VERSION:6

# 1080p 렌디션 정보
#EXT-X-STREAM-INF:BANDWIDTH=5000000,AVERAGE-BANDWIDTH=4500000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
1080p/playlist.m3u8

# 720p 렌디션 정보
#EXT-X-STREAM-INF:BANDWIDTH=2800000,AVERAGE-BANDWIDTH=2500000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
720p/playlist.m3u8
...
```

한편, 재생목록 생성기가 특정 렌디션의 미디어 재생목록을 생성할 때 다른 모든 렌디션의 최신 상태를 세션 관리자에 쿼리하여 EXT-X-RENDITION-REPORT 태그들을 삽입하도록 해야 한다. 다음 예시는 720p가 아닌 스트림을 제공하는 도중 720p 스트림 파트 생성 현황을 클라이언트에게 알린다.

```text
...
# e.g. 720p 렌디션의 최신 상태 보고
#EXT-X-RENDITION-REPORT:URI="../720p/playlist.m3u8",LAST-MSN=100,LAST-PART=2
```

ABR 및 Rendition Report 기능은 세션 관리자의 중앙 상태 관리와 재생목록 생성기의 동적 생성 로직 간의 협력을 통해 구현된다. 클라이언트가 마스터 재생목록을 요청하면 HTTP/3 요청 핸들러는 이를 재생목록 생성기에 위임하고 생성기는 서버가 지원하는 렌디션 목록을 기반으로 EXT-X-STREAM-INF 태그들을 포함한 마스터 재생목록들을 생성해 응답한다. 이후 클라이언트가 특정 렌디션의 미디어 재생목록을 요청하면 재생목록 생성기는 세션 관리자에게 해당 VOD 세션의 모든 렌디션에 대한 최신 LAST-MSN 속성과 LAST-PART 속성 정보를 요청한다. 이 때 세션 관리자는 각 렌디션 별로 진행 중인 인코딩 상태를 추적하고 있으므로 해당 요청에 대한 정확한 상태 정보를 즉시 반환할 수 있다. 재생목록 생성기는 이 정보를 받아 스트리밍 중인 스트림의 파트 목록과 함께 다른 렌디션들에 대한 EXT-X-RENDITION-REPORT 태그들을 조합해 최종 재생목록을 완성하고 클라이언트에게 전송한다. 본 논문에서는 이러한 아키텍처를 통해 각 렌디션 상태를 Manager, ABRStream 모델, Handler 간 유기적 협력을 통해 관리하여 LL-HLS 환경에서 발생할 수 있는 동기화 문제를 해결하고 안정적이고 부드러운 적응형 스트리밍 경험을 제공한다.

# 결론

본 논문은 VOD 서비스의 사용자 경험 향상을 위해 기존 HTTP 기반 스트리밍 기술의 전송 계층 측면의 한계를 극복하고자 차세대 전송 프로토콜인 QUIC(over HTTP/3)을 저지연 스트리밍 기술인 LL-HLS와 결합한 VOD 서비스 전용 서버 시스템을 구현했다. 이는 아직 프로덕션 환경에서 널리 활용되지 않는 HTTP/3 전송 프로토콜 위에서 복잡성을 가지는 LL-HLS의 동작을 구현 및 연동함으로써 기술적 타당성을 보여준다는 의미가 있다. 구현 과정에서 LL-HLS 관련 오픈소스의 부재 등의 기술적 도전 과제에 직면했으며 이를 해결하여 Smart Origin Server 구성에 대한 실질적인 이해를 얻을 수 있었다. 해당 과정에서 얻은 기술적 경험과 시사점이 향후 실제 서비스에 HTTP/3 전송 프로토콜을 적용할 때 기반 정보를 제공할 수 있음을 기대한다. 또한 본 연구를 기반으로 다음과 같은 후속 연구를 제안한다. 첫째로, 본 논문에서 다루지 않은 실시간 스트리밍 환경으로 시스템을 확장하는 것이다. VOD와 달리 예측 불가능한 라이브 콘텐츠를 QUIC과 LL-HLS로 안정적으로 전송하는 데 발생하는 추가적인 기술적 과제를 해결하고 그 효용성을 검증하는 과정이 필요하다. 둘째로, 정량적인 성능 평가 고도화가 필요하다. 다양한 네트워크 시뮬레이션을 통해 패킷 손실률, RTT(Round-Trip Time) 변화 등 열악한 환경에서 기존의 HTTP/1.1 및 HTTP/2 기반 HLS/DASH 시스템 대비 지연 시간, 끊김 현상(Rebuffering), 리소스 사용률 측면에서 얼마나 우위를 갖는지 구체적인 수치 증명이 필요하다.

---

[^1]: Woo, J., Hong, S., Kang, D., & An, D. (2024). Improving the Quality of Experience of Video Streaming Through a Buffer-Based Adaptive Bitrate Algorithm and Gated Recurrent Unit-Based Network Bandwidth Prediction. Applied Sciences, 14(22), 10490.
[^2]: Gañán, C. H. (2009). Scalable Multi-Source Video Streaming Application over Peer-to-Peer Networks. Master's thesis, Polytechnical University of Catalonia.
[^3]: Vossen, G., & Hagemann, S. (2007). From Version 1.0 to Version 2.0: A brief history of the web. (ERCIS Working Paper, No. 4). European Research Center for Information Systems (ERCIS), University of Münster.
[^4]: 박지우, & 정광수. (2017). 저지연 라이브 HTTP 스트리밍을 위한 전송률 적응 기법. 2017년 한국컴퓨터종합학술대회 논문집, 1187-1189.
[^5]: Schulzrinne, H., Rao, A., & Lanphier, R. (1998). Real Time Streaming Protocol (RTSP). RFC 2326, Standards Track.
[^6]: Pantos, R. (Ed.), & May, W. (2017). HTTP Live Streaming. RFC 8216, Informational.
[^7]: 김인기, & 강민구. (2016). 적응 버퍼링 성능분석 기반의 스마트 OTT 플랫폼 설계. Journal of Internet Computing and Services (JICS), 17(4), 19-26.
[^8]: 심재훈, 김하영, 박노현, 박예빈, Enenche, P., 신광무, 김성훈, & 유동호. (2022). HLS(HTTP Live Streaming)에서 조건부 대체 알고리즘 적용을 위한 고찰. 2022년 한국정보기술학회 하계종합학술대회 논문집.
[^9]: Trevisan, M., Giordano, D., Drago, I., & Khatouni, A. S. (2021). Measuring HTTP/3: Adoption and Performance. 19th Mediterranean Communication and Computer Networking Conference (MedComNet).
[^10]: Liu, F., Dehart, J., Parwatikar, J., Farkiani, B., & Crowley, P. (2024). Performance Comparison of HTTP/3 and HTTP/2: Proxy vs. Non-Proxy Environments. arXiv preprint arXiv:2409.16267.
[^11]: Khan, K. (2023). Enhancing Adaptive Video Streaming through Fuzzy Logic-Based Content Recommendation Systems: A Comprehensive Review and Future Directions.
