# Live business-language brief → generated journey → Test Lab

Recorded 2026-11-12 against source `27438fa`. A real OpenAI `gpt-6-luna`
Composer request (`composer.v3`) generated one new journey in the local demo
workspace. **10 of 11 scenarios passed on the unmodified model output.** The
remaining boundary case failed, and the publication check returned
`ready=false`, `test_failed`. No version was published or real message sent.

[Full provider accounting, proposal, scenarios, assertions, rendered messages and traces](../validation/live-brief-cart-2026-11-12/result.json).
[Review screenshot](../validation/live-brief-cart-2026-11-12/review.png).
[Canvas screenshot](../validation/live-brief-cart-2026-11-12/canvas.png).

## Open the saved draft

Journey: **AI · Sepet Geri Kazanım — Canlı Brief Testi**.

[Open in the local application](http://localhost:5173/journeys/df88e290-d6e0-489a-bea8-8b42f1c843d1). Sign in with the normal demo login.
The generated draft and all eleven sandbox runs remain available. The output was
applied only to prepare the requested sandbox evaluation, with the boundary
problem recorded at review; this is not production approval.

## Brief supplied to the model

The request used business language without technical event/column names:

Sepetinde en az 50 EUR değerinde ürün bırakıp satın almayan müşterileri geri kazanmak istiyorum. Müşterinin bu sepeti zaten satın alınmışsa mesaj göndermeden başarılı tamamla. Hiçbir iletişim kanalına izin yoksa hemen uygun değil olarak sonlandır.
Bir saat bekle; güncel satın alma bilgisine tekrar bak. Satın almamış ve e-posta izni varsa müşterinin adını ve sepet tutarını kullanarak hatırlatma e-postası gönder. E-posta izni yoksa bu mesajı atla ama izinli SMS yoluna devam et.
24 saat daha bekle; güncel satın alma bilgisine tekrar bak. Hâlâ satın almamış ve SMS izni varsa COMEBACK10 koduyla yüzde 10 indirim sunan kişiselleştirilmiş SMS gönder. SMS izni yoksa dönüşümsüz tamamla. SMS gönderilirse 48 saat daha bekle ve güncel satın alma bilgisine göre başarılı ya da dönüşümsüz tamamla.
Satın alma olayı hangi aşamada gelirse gelsin kalan adımları hemen durdur ve dönüşüm olarak say. Aynı müşteri yedi gün dolmadan yeniden giremesin. Eksik ad ve tutarda anlamlı yedek metin kullan. Sepete dönüş adresi şimdilik https://example.com/cart; yayından önce gerçek adresin ve indirim kodunun doğrulanması gerektiğini belirt. Mesajlar Türkçe olsun.

## Generation and review

The demo workspace initially had only built-in catalog entries. Explicit bounded
catalog discovery read existing local demo metadata. Existing cart/purchase,
amount/currency, first-name and current-cart-purchased mappings received business
labels. No live customer values were supplied to the LLM. All generated references
matched the resulting catalog. This setup is included in the recorded end-to-end
time, and is not evidence that generation works without a workspace catalog.

One generation request and **one provider attempt** succeeded in
**33.90 seconds**. The model produced **16 nodes and 20 edges**.
Graph/message validation passed. The workflow contains:

1. Cart threshold and EUR filter, then an existing-purchase check.
2. Immediate ineligible exit without channel consent.
3. One-hour wait, current purchase check and consent-gated email.
4. Both email-sent and email-skipped paths merge into a 24-hour wait.
5. Current purchase check, consent-gated discount SMS or normal completion.
6. After SMS, a 48-hour wait and final current purchase check.
7. Explicit converted global purchase exit and seven-day re-entry cooldown.

The model warned about the placeholder URL/coupon, provider configuration and
lack of cart-ID correlation on the global purchase exit. That last warning is
material: an unrelated purchase by the same customer can exit this draft. The
current test uses a matching cart and does not prove cross-cart isolation. A
selected business acceptance contract was not supplied; scenarios, rather than
graph validity, evaluate brief compliance here.

## Sandbox results

Virtual hours from entry; message counts represent simulated logical messages.
No production provider was invoked. All test profiles/contact values are fictional.

| Scenario                                    | Result | Email / SMS | Virtual hours | Outcome           |
| ------------------------------------------- | ------ | ----------: | ------------: | ----------------- |
| A · Satın alma yok: email ve SMS            | PASS   |       1 / 1 |            73 | completed         |
| B · 30 dakikada satın alma: mesaj yok       | PASS   |       0 / 0 |           0.5 | converted         |
| C · Email sonrası satın alma: SMS yok       | PASS   |       1 / 0 |             2 | converted         |
| D · Email izni yok: SMS yolu korunur        | PASS   |       0 / 1 |            73 | completed         |
| E · SMS izni yok: yalnızca email            | PASS   |       1 / 0 |            25 | completed         |
| F · Hiçbir kanal izni yok: anında çıkış     | PASS   |       0 / 0 |             0 | condition_not_met |
| G · Önceden satın alınmış sepet             | PASS   |       0 / 0 |             0 | converted         |
| H · 49,995 EUR: giriş yapılmamalı           | FAIL   |       1 / 1 |            73 | completed         |
| I · Tam 50 EUR: giriş yapılmalı             | PASS   |       1 / 1 |            73 | completed         |
| J · Profil 24. saatte satın alındı: SMS yok | PASS   |       1 / 0 |            25 | converted         |
| K · Email provider 429 → 500 → başarı       | PASS   |       1 / 1 |            73 | completed         |

The fault-injection trace records the initial 429 at email scheduling, a 500
retry at +2 virtual seconds and acceptance at +6 seconds. The case retained one
logical email. Every case also asserted no duplicate logical
delivery. Real-provider retry/delivery guarantees are outside this sandbox test.

The happy-path email rendered “Merhaba Alex” with “89,90 EUR”; the SMS included
COMEBACK10, %10 and the demo URL. Email HTML was sanitized. The Turkish SMS was
classified Unicode with three segments. Missing-value fallbacks and the seven-day
cooldown are present in the saved definition, but these eleven cases do not
exercise missing-value or repeated-entry behavior.

## Boundary failure

The model translated “at least 50 EUR” as **amount > 49.99**. With a numeric amount
of **49.995**, expected enrollment/message counts were zero; actual enrollment
was one and email/SMS counts were one each. Exactly 50 correctly entered.
The recorded 49.995 case intentionally changes the numeric entry filter input;
its display amount remains the base fixture string and is not used for threshold
evaluation.

This defect was visible in review and reproduced without editing the graph.
Graph/schema/catalog validation all passed: they establish structural validity,
not the numeric equivalence of a natural-language threshold. The failed required
scenario correctly blocked publication. The needed representation is
**amount > 50 OR amount == 50**, combined with the EUR constraint, or an explicit
supported greater-than-or-equal operator. No automatic correction or extra paid
request was made in this run. This is a recurring failure also observed in the
prior semantic audit; it is not reported as newly fixed.

## Canvas observation

The initial builder capture exposes a separate presentation problem: model-supplied
horizontal positions are close enough that some cards overlap, and the initial
viewport leaves considerable empty space to the left. Review is readable, but
this trial does not establish a clean canvas layout. The generated coordinates
were preserved with the rest of the proposal rather than silently rearranged to
make the evidence look better. Display-time layout and initial focus should be
checked separately from graph execution correctness.

## Timing and usage

| Item                                                                 |  Actual |
| -------------------------------------------------------------------- | ------: |
| Generation HTTP wall time                                            | 33.90 s |
| Eleven run creation/advance/inbox-detail sequences, summed wall time | 0.137 s |
| Recorded setup → review → sandbox completion                         | 82.50 s |
| Provider input tokens                                                |   3,489 |
| Provider output tokens                                               |   3,367 |
| Input + output                                                       |   6,856 |
| Reported reasoning tokens, included in output                        |   1,484 |

End-to-end time includes catalog setup, browser navigation/screenshots and the
review pause; it is not model latency alone. The raw provider usage fields are
preserved without converting token counts into a guessed monetary charge.
No second generation, repair request or workflow patch was performed.

## Isolation and status

The journey remains Draft with zero published versions and zero live executions.
The workspace's live delivery count did not increase. All produced inbox messages
were SIMULATED and scoped to their run. The saved workflow equals the original
provider proposal. No API secret, password or session token is included in these
artifacts. This is a real paid generation plus deterministic sandbox evaluation,
not evidence that the draft is ready to publish.
