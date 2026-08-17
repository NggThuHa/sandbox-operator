<!--
EN: Keep the sections you need and delete the rest. A pull request that explains
    *why* gets reviewed faster than one that only shows *what*.
VI: Giữ lại phần bạn cần và xoá phần còn lại. Một pull request giải thích được *vì
    sao* sẽ được review nhanh hơn loại chỉ cho thấy *cái gì*.
-->

## Summary | Tóm tắt

<!-- What changes, in one or two sentences. | Thay đổi gì, trong một hai câu. -->

## Why | Vì sao

<!--
The problem, not the patch. Link the issue if there is one: "Closes #123".
Vấn đề, không phải bản vá. Kèm link issue nếu có: "Closes #123".
-->

## Type of change | Loại thay đổi

- [ ] Bug fix | Sửa lỗi
- [ ] New feature | Tính năng mới
- [ ] Refactor / cleanup | Tái cấu trúc / dọn dẹp
- [ ] Docs | Tài liệu
- [ ] CI / infrastructure | CI / hạ tầng
- [ ] Breaking change | Thay đổi phá vỡ tương thích

## API impact | Ảnh hưởng tới API

<!--
Did api/v1alpha1 change? Then say what happens to existing ClusterLab /
InstanceLab objects on an upgrade.
api/v1alpha1 có thay đổi không? Nếu có, hãy nói rõ các object ClusterLab /
InstanceLab đang tồn tại sẽ thế nào khi nâng cấp.
-->

- [ ] No API change | Không đổi API
- [ ] API changed and `make manifests generate` was run and committed | Có đổi API, đã chạy và commit `make manifests generate`

## Verification | Đã kiểm chứng

<!--
Commands you actually ran, with the outcome. "make test passes" beats a checkbox.
Các lệnh bạn thực sự đã chạy, kèm kết quả. "make test passes" có giá trị hơn một ô tick.
-->

```console
$ make lint test
```

- [ ] `make lint test` | Lint và unit test
- [ ] `make test-e2e` (Kind) — required for controller changes | bắt buộc khi sửa controller
- [ ] Ansible: `ansible-lint cluster.yml` / `molecule test` — for `infra/ansible/**` | cho `infra/ansible/**`
- [ ] Terraform: `terraform fmt -check -recursive` + `terraform validate` — for `infra/terraform/**` | cho `infra/terraform/**`
- [ ] Tried on a real cluster | Đã thử trên cụm thật

## Notes for the reviewer | Ghi chú cho người review

<!--
Anything deliberately left out, a trade-off you are unsure about, a follow-up you
plan to open.
Điều gì cố ý bỏ ra ngoài phạm vi, một đánh đổi bạn còn chưa chắc, một việc bạn dự
định làm tiếp.
-->
