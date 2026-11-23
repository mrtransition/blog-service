# Safetensors 转 GGUF 操作手册（使用 llama.cpp）

本手册介绍如何将 Hugging Face 格式的模型（通常包含 `.safetensors` 权重文件）转换为 GGUF 格式，以便在 llama.cpp 中高效运行。转换过程使用 llama.cpp 提供的官方脚本，支持大多数常见模型架构（如 LLaMA、Mistral、Qwen、ChatGLM 等）。

---

## 1. 背景知识

- **Safetensors**：由 Hugging Face 推出的一种安全、快速的模型权重存储格式，常用于 PyTorch 模型。
- **GGUF**：llama.cpp 使用的统一模型格式，支持量化、内存映射和多种推理后端，适合在 CPU/GPU 上高效运行。
- **转换必要性**：llama.cpp 无法直接加载 safetensors，需要将模型权重、配置和词表打包为 GGUF 文件。

---

## 2. 准备工作

### 2.1 环境要求

- Python 3.8+
- Git
- 足够的磁盘空间（转换和量化可能产生大文件）
- （可选）CUDA 或其他加速库

### 2.2 获取 llama.cpp

```bash
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
```

llama.cpp 的转换脚本位于根目录，无需编译即可运行（仅使用 Python）。但后续若需量化或推理，需编译项目：

```bash
make -j
# 或启用 GPU 支持，如：make LLAMA_CUDA=1 -j
```

### 2.3 准备模型文件

确保你拥有完整的 Hugging Face 模型目录，通常包含以下文件：

```
model_name/
├── config.json
├── tokenizer.json / tokenizer.model
├── model-00001-of-00002.safetensors
├── model-00002-of-00002.safetensors
├── model.safetensors.index.json
└── ...
```

如果模型以 `.bin` 格式存储，转换方法相同（脚本会自动识别）。

---

## 3. 转换步骤

### 3.1 使用 `convert_hf_to_gguf.py` 脚本

llama.cpp 提供了 `convert_hf_to_gguf.py` 脚本（旧版为 `convert.py`），可直接处理 Hugging Face 格式。执行以下命令：

```bash
python convert_hf_to_gguf.py /path/to/hf_model --outfile /path/to/output.gguf --outtype f16
```

**参数说明**：

- 第一个位置参数：Hugging Face 模型目录路径。
- `--outfile`：输出 GGUF 文件路径。
- `--outtype`：输出权重精度，可选 `f32`、`f16`、`bf16`、`q8_0` 等。推荐先用 `f16`（默认），后续可再进行量化。

**示例**：

```bash
python convert_hf_to_gguf.py ./models/Llama-2-7b-hf --outfile ./models/llama-2-7b.gguf --outtype f16
```

转换过程会输出模型架构、层数等信息，完成后生成 `.gguf` 文件。

### 3.2 处理特殊模型

- 如果模型使用了自定义 tokenizer（如 SentencePiece 的 `.model` 文件），脚本通常会自动处理；若失败，可手动指定 tokenizer 路径。
- 对于需要额外配置的模型（如 ChatGLM），请查看脚本支持列表或添加 `--verbose` 调试。

---

## 4. 模型量化（可选）

为了进一步缩小模型体积并加速推理，可以对 GGUF 文件进行量化。llama.cpp 编译后提供 `llama-quantize` 工具。

```bash
./llama-quantize /path/to/input.gguf /path/to/output-quantized.gguf Q4_K_M
```

**常用量化类型**：

| 类型   | 描述                     | 大小（7B 模型参考） |
|--------|--------------------------|---------------------|
| Q4_0   | 4-bit 量化，速度较快     | ~3.8 GB             |
| Q4_K_M | 4-bit 量化，质量较好     | ~4.1 GB             |
| Q5_K_M | 5-bit 量化，质量更高     | ~4.8 GB             |
| Q8_0   | 8-bit 量化，接近无损     | ~7.2 GB             |

建议根据硬件资源和精度要求选择。

---

## 5. 验证转换结果

使用 llama.cpp 的推理工具测试 GGUF 文件是否可用：

```bash
./main -m /path/to/model.gguf -p "Hello, world!" -n 32
```

若正常输出文本，则转换成功。

---

## 6. 常见问题与解决

### 6.1 报错：`KeyError: 'model.embed_tokens.weight'`

- 原因：模型权重名称与脚本预期不符（例如使用了 `model.layers` 而非 `transformer.h`）。
- 解决：尝试更新 llama.cpp 到最新版，或修改脚本中的映射逻辑（需要一定技术）。

### 6.2 输出文件巨大

- 原因：使用了 `f32` 输出且未量化。建议先转 `f16`，再根据需要量化。

### 6.3 转换后推理结果异常

- 确保使用了正确的 prompt 模板和特殊 token（如 BOS/EOS）。部分模型需要设置 `--chat-template` 参数。

### 6.4 脚本找不到 `transformers` 库

- 部分模型需要 Hugging Face `transformers` 库加载配置。执行 `pip install transformers`。

---

## 7. 总结

通过 llama.cpp 官方脚本，将 safetensors 模型转换为 GGUF 格式仅需一条命令。转换后可根据需求量化，并使用 llama.cpp 进行高效推理。如果遇到不支持的模型架构，请查阅 llama.cpp 仓库的 Issues 或提交问题。

---

## 8. 参考链接

- [llama.cpp GitHub](https://github.com/ggerganov/llama.cpp)
- [GGUF 格式说明](https://github.com/ggerganov/ggml/blob/master/docs/gguf.md)
- [Hugging Face 模型库](https://huggingface.co/models)

---

本手册基于 llama.cpp 2024 年最新版本编写，如有更新请以官方文档为准。